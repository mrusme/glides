package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	_ "modernc.org/sqlite"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
)

const (
	DefaultMigrationsPath string = "migrations"
	DefaultMaxOpenConns   int    = 4
	driverName            string = "sqlite"
)

var memories atomic.Uint64

var pathEscaper = strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23")

type Options struct {
	Path           string
	InMemory       bool
	ReadOnly       bool
	MaxOpenConns   int
	Pragmas        []string
	Migrations     *embed.FS
	MigrationsPath string
}

type driver struct {
	log   *slog.Logger
	opts  Options
	dsn   string
	sqldb *sql.DB
	open  bool
}

func New(log *slog.Logger, opts Options) (db *database.Database, err error) {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.MigrationsPath == "" {
		opts.MigrationsPath = DefaultMigrationsPath
	}
	if opts.MaxOpenConns <= 0 {
		opts.MaxOpenConns = DefaultMaxOpenConns
	}

	switch {
	case opts.InMemory && opts.Path != "":
		return nil, fmt.Errorf("%w: a path and in memory", errs.ErrDatabaseOptionsInvalid)
	case !opts.InMemory && opts.Path == "":
		return nil, fmt.Errorf("%w: no path", errs.ErrDatabaseOptionsInvalid)
	case opts.Path == ":memory:":
		return nil, fmt.Errorf("%w: :memory: as the path", errs.ErrDatabaseOptionsInvalid)
	case opts.ReadOnly && opts.InMemory:
		return nil, fmt.Errorf("%w: read-only and in memory", errs.ErrDatabaseOptionsInvalid)
	case opts.ReadOnly && opts.Migrations != nil:
		return nil, fmt.Errorf("%w: read-only with migrations", errs.ErrDatabaseOptionsInvalid)
	}

	d := new(driver)
	d.log = log
	d.opts = opts
	d.dsn = dsn(opts)

	return database.New(log, d), nil
}

func dsn(opts Options) string {
	q := url.Values{}
	q.Set("_busy_timeout", strconv.FormatInt(database.DefaultTimeout.Milliseconds(), 10))
	q.Set("_foreign_keys", "1")
	q.Set("_time_format", "sqlite")
	q.Set("_timezone", "UTC")

	name := pathEscaper.Replace(opts.Path)
	switch {
	case opts.InMemory:
		name = fmt.Sprintf("memory-%d", memories.Add(1))
		q.Set("mode", "memory")
		q.Set("cache", "shared")
		q.Set("_txlock", "immediate")
	case opts.ReadOnly:
		q.Set("mode", "ro")
	default:
		q.Set("_journal_mode", "WAL")
		q.Set("_synchronous", "NORMAL")
		q.Set("_txlock", "immediate")
	}

	for _, pragma := range opts.Pragmas {
		q.Add("_pragma", pragma)
	}

	return "file:" + name + "?" + q.Encode()
}

func (d *driver) Open(ctx context.Context) (err error) {
	sqldb, err := sql.Open(driverName, d.dsn)
	if err != nil {
		return ConvertError(err)
	}
	sqldb.SetMaxOpenConns(d.opts.MaxOpenConns)
	sqldb.SetMaxIdleConns(d.opts.MaxOpenConns)

	pingCtx, cancel := context.WithTimeout(ctx, database.StartupTimeout)
	defer cancel()

	if err = sqldb.PingContext(pingCtx); err != nil {
		sqldb.Close()
		return ConvertError(err)
	}

	if d.opts.Migrations == nil {
		d.log.Info("Database.Startup", "migrations", "none")
	} else if err = d.migrate(); err != nil {
		sqldb.Close()
		return err
	}

	d.sqldb = sqldb
	d.open = true

	return nil
}

func (d *driver) Close() (err error) {
	if d.sqldb != nil {
		err = d.sqldb.Close()
	}
	d.open = false

	return ConvertError(err)
}

func (d *driver) Ping(ctx context.Context) error {
	if d.sqldb == nil {
		return errs.ErrDatabaseNotStarted
	}

	return ConvertError(d.sqldb.PingContext(ctx))
}

func (d *driver) Reset(ctx context.Context) (err error) {
	if d.opts.ReadOnly {
		return errs.ErrDatabaseReadOnly
	}

	sqldb := d.sqldb
	if !d.open {
		if sqldb, err = sql.Open(driverName, d.dsn); err != nil {
			return ConvertError(err)
		}
		defer sqldb.Close()
	}

	tx, err := sqldb.BeginTx(ctx, nil)
	if err != nil {
		return ConvertError(err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, "PRAGMA defer_foreign_keys = ON"); err != nil {
		return ConvertError(err)
	}

	objects, err := tx.QueryContext(ctx, `SELECT type, name FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY CASE type WHEN 'view' THEN 0 ELSE 1 END, name`)
	if err != nil {
		return ConvertError(err)
	}

	var statements []string
	for objects.Next() {
		var kind, name string
		if err = objects.Scan(&kind, &name); err != nil {
			objects.Close()
			return ConvertError(err)
		}
		statements = append(statements, "DROP "+strings.ToUpper(kind)+` IF EXISTS "`+strings.ReplaceAll(name, `"`, `""`)+`"`)
	}
	objects.Close()
	if err = objects.Err(); err != nil {
		return ConvertError(err)
	}

	for _, statement := range statements {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return ConvertError(err)
		}
	}

	if err = tx.Commit(); err != nil {
		return ConvertError(err)
	}

	_, err = sqldb.ExecContext(ctx, "VACUUM")

	return ConvertError(err)
}
