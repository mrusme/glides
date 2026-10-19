package database

import (
	"context"

	"xn--gckvb8fzb.com/glides/errs"
)

type Tx struct {
	db *Database
	tx DriverTx
}

func (db *Database) Tx(ctx context.Context) (tx *Tx, err error) {
	if !db.started.Load() {
		return nil, errs.ErrDatabaseNotStarted
	}

	ctx, cancel := db.deadline(ctx)
	defer cancel()

	tx = new(Tx)
	tx.db = db

	if tx.tx, err = db.driver.Begin(ctx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (tx *Tx) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (Result, error) {
	ctx, cancel := tx.db.deadline(ctx)
	defer cancel()

	return tx.tx.Exec(ctx, sql, args...)
}

func (tx *Tx) Query(
	ctx context.Context,
	sql string,
	args ...any,
) (Cursor, error) {
	ctx, cancel := tx.db.deadline(ctx)

	result, err := tx.tx.Query(ctx, sql, args...)
	if err != nil {
		cancel()
		return nil, err
	}

	return &cursor{Cursor: result, cancel: cancel}, nil
}

func (tx *Tx) QueryRow(
	ctx context.Context,
	sql string,
	args ...any,
) Scanner {
	ctx, cancel := tx.db.deadline(ctx)

	return &scanner{Scanner: tx.tx.QueryRow(ctx, sql, args...), cancel: cancel}
}

func (tx *Tx) Commit(ctx context.Context) error {
	ctx, cancel := tx.db.deadline(ctx)
	defer cancel()

	return tx.tx.Commit(ctx)
}

func (tx *Tx) End(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tx.db.timeout)
	defer cancel()

	return tx.tx.Rollback(ctx)
}
