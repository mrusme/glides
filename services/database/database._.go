package database

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"xn--gckvb8fzb.com/glides/errs"
)

const (
	DefaultTimeout time.Duration = 10 * time.Second
	StartupTimeout time.Duration = 5 * time.Second
	ResetTimeout   time.Duration = 30 * time.Second
)

var nopCancel context.CancelFunc = func() {}

type Result interface {
	RowsAffected() int64
}

type Cursor interface {
	Next() bool
	Scan(dest ...any) error
	Columns() []string
	Err() error
	Close()
}

type Scanner interface {
	Scan(dest ...any) error
}

type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (Result, error)
	Query(ctx context.Context, sql string, args ...any) (Cursor, error)
	QueryRow(ctx context.Context, sql string, args ...any) Scanner
}

type Driver interface {
	Executor
	Open(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error
	Reset(ctx context.Context) error
	Begin(ctx context.Context) (DriverTx, error)
}

type DriverTx interface {
	Executor
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Database struct {
	log     *slog.Logger
	driver  Driver
	timeout time.Duration
	started atomic.Bool
}

func New(log *slog.Logger, driver Driver) (db *Database) {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	db = new(Database)
	db.log = log
	db.driver = driver
	db.timeout = DefaultTimeout

	return db
}

func (db *Database) Startup() (err error) {
	if err = db.driver.Open(context.Background()); err != nil {
		return err
	}

	db.started.Store(true)

	return nil
}

func (db *Database) Shutdown() error {
	if !db.started.Swap(false) {
		return nil
	}

	return db.driver.Close()
}

func (db *Database) Ping(ctx context.Context) error {
	if !db.started.Load() {
		return errs.ErrDatabaseNotStarted
	}

	ctx, cancel := db.deadline(ctx)
	defer cancel()

	return db.driver.Ping(ctx)
}

func (db *Database) Reset() (err error) {
	db.log.Warn("Database.Reset", "status", "clearing database")

	ctx, cancel := context.WithTimeout(context.Background(), ResetTimeout)
	defer cancel()

	if err = db.driver.Reset(ctx); err != nil {
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
