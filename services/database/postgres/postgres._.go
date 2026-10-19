package postgres

import (
	"context"
	"embed"
	"io"
	"log/slog"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
)

const DefaultMigrationsPath string = "migrations"

type Options struct {
	Migrations     *embed.FS
	MigrationsPath string
}

type driver struct {
	log     *slog.Logger
	opts    Options
	poolcfg *pgxpool.Config
	pool    *pgxpool.Pool
}

func New(
	log *slog.Logger,
	connection string,
	opts Options,
) (db *database.Database, err error) {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.MigrationsPath == "" {
		opts.MigrationsPath = DefaultMigrationsPath
	}

	d := new(driver)
	d.log = log
	d.opts = opts

	if d.poolcfg, err = pgxpool.ParseConfig(connection); err != nil {
		return nil, err
	}

	d.poolcfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		pgxUUID.Register(c.TypeMap())
		return nil
	}

	if log.Enabled(context.Background(), slog.LevelDebug) {
		d.poolcfg.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   tracelog.LoggerFunc(d.trace),
			LogLevel: tracelog.LogLevelDebug,
		}
	}

	return database.New(log, d), nil
}

func (d *driver) trace(
	ctx context.Context,
	level tracelog.LogLevel,
	msg string,
	data map[string]any,
) {
	attrs := make([]any, 0, 2*len(data))
	for _, key := range slices.Sorted(maps.Keys(data)) {
		if key == "args" {
			continue
		}
		attrs = append(attrs, key, data[key])
	}

	d.log.Debug("Database."+msg, attrs...)
}

func (d *driver) newPool(ctx context.Context) (*pgxpool.Pool, error) {
	return pgxpool.NewWithConfig(ctx, d.poolcfg)
}

func (d *driver) Open(ctx context.Context) (err error) {
	if d.pool, err = d.newPool(ctx); err != nil {
		d.pool = nil
		return err
	}

	if err = d.ping(ctx); err != nil {
		d.Close()
		return err
	}

	if d.opts.Migrations == nil {
		d.log.Info("Database.Startup", "migrations", "none")
		return nil
	}

	if err = d.migrate(); err != nil {
		d.Close()
		return err
	}

	return nil
}

func (d *driver) ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, database.StartupTimeout)
	defer cancel()

	return d.pool.Ping(ctx)
}

func (d *driver) Close() error {
	if d.pool != nil {
		d.pool.Close()
	}

	return nil
}

func (d *driver) Ping(ctx context.Context) error {
	if d.pool == nil {
		return errs.ErrDatabaseNotStarted
	}

	return ConvertError(d.pool.Ping(ctx))
}

func (d *driver) Reset(ctx context.Context) (err error) {
	pool, err := d.newPool(ctx)
	if err != nil {
		return ConvertError(err)
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		return ConvertError(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return ConvertError(err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, "DROP SCHEMA public CASCADE"); err != nil {
		return ConvertError(err)
	}
	if _, err = tx.Exec(ctx, "CREATE SCHEMA public"); err != nil {
		return ConvertError(err)
	}

	return ConvertError(tx.Commit(ctx))
}
