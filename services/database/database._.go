package database

import (
	"context"
	"embed"
	"io"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/tracelog"
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"
)

const (
	DefaultTimeout time.Duration = 10 * time.Second
	StartupTimeout time.Duration = 5 * time.Second
	ResetTimeout   time.Duration = 30 * time.Second
)

var nopCancel context.CancelFunc = func() {}

type Database struct {
	log        *slog.Logger
	connection string
	migrations *embed.FS
	timeout    time.Duration

	poolcfg *pgxpool.Config
	pool    *pgxpool.Pool
}

func New(
	log *slog.Logger,
	connection string,
) (db *Database, err error) {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	db = new(Database)
	db.log = log
	db.connection = connection
	db.timeout = DefaultTimeout

	if db.poolcfg, err = pgxpool.ParseConfig(connection); err != nil {
		return nil, err
	}

	db.poolcfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		pgxUUID.Register(c.TypeMap())
		return nil
	}

	if log.Enabled(context.Background(), slog.LevelDebug) {
		db.poolcfg.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   tracelog.LoggerFunc(db.trace),
			LogLevel: tracelog.LogLevelDebug,
		}
	}

	return db, nil
}

func (db *Database) trace(
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

	db.log.Debug("Database."+msg, attrs...)
}

func (db *Database) SetMigrations(migrations *embed.FS) {
	db.migrations = migrations
}

func newPool(db *Database) (*pgxpool.Pool, error) {
	return pgxpool.NewWithConfig(context.Background(), db.poolcfg)
}

func (db *Database) Startup() (err error) {
	if db.pool, err = newPool(db); err != nil {
		db.pool = nil
		return err
	}

	if err = db.ping(); err != nil {
		db.Shutdown()
		return err
	}

	if db.migrations == nil {
		db.log.Info("Database.Startup", "migrations", "none")
		return nil
	}

	if err = db.migrate(); err != nil {
		db.Shutdown()
		return err
	}

	return nil
}

func (db *Database) ping() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), StartupTimeout)
	defer cancel()

	return db.pool.Ping(ctx)
}

func (db *Database) migrate() (err error) {
	source, err := iofs.New(db.migrations, "migrations")
	if err != nil {
		return err
	}

	sqldb := stdlib.OpenDBFromPool(db.pool)

	driver, err := migratepgx.WithInstance(sqldb, &migratepgx.Config{})
	if err != nil {
		source.Close()
		sqldb.Close()
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		source.Close()
		driver.Close()
		return err
	}

	err = m.Up()
	serr, dberr := m.Close()

	switch {
	case err != nil && err != migrate.ErrNoChange:
		return err
	case serr != nil:
		return serr
	}

	return dberr
}

func (db *Database) Shutdown() error {
	if db.pool != nil {
		db.pool.Close()
	}

	return nil
}

func (db *Database) Reset() (err error) {
	db.log.Warn("Database.Reset", "status", "clearing database")

	pool, err := newPool(db)
	if err != nil {
		return err
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), ResetTimeout)
	defer cancel()

	if err = pool.Ping(ctx); err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, "DROP SCHEMA public CASCADE"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE SCHEMA public"); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	db.log.Info("Database.Reset", "status", "ok")

	return nil
}

func (db *Database) deadline(
	ctx context.Context,
) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, nopCancel
	}

	return context.WithTimeout(ctx, db.timeout)
}
