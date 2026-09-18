package database

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"
)

const (
	DefaultTimeout time.Duration = 10 * time.Second
	StartupTimeout time.Duration = 5 * time.Second
	ResetTimeout   time.Duration = 30 * time.Second
)

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

	return db, nil
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
	if db.pool, err = newPool(db); err != nil {
		db.pool = nil
		return err
	}
	defer db.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), ResetTimeout)
	defer cancel()

	if err = db.pool.Ping(ctx); err != nil {
		return err
	}

	tx, err := db.pool.Begin(ctx)
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

	return tx.Commit(ctx)
}

func (db *Database) deadline(
	ctx context.Context,
) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, db.timeout)
}
