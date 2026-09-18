package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"xn--gckvb8fzb.com/glides/errs"
)

type Tx struct {
	db *Database
	tx pgx.Tx
}

func (db *Database) Tx(ctx context.Context) (tx *Tx, err error) {
	if db.pool == nil {
		return nil, errs.ErrDatabaseNotStarted
	}

	ctx, cancel := db.deadline(ctx)
	defer cancel()

	tx = new(Tx)
	tx.db = db

	if tx.tx, err = db.pool.Begin(ctx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (tx *Tx) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (pgconn.CommandTag, error) {
	ctx, cancel := tx.db.deadline(ctx)
	defer cancel()

	return tx.tx.Exec(ctx, sql, args...)
}

func (tx *Tx) Query(
	ctx context.Context,
	sql string,
	args ...any,
) (pgx.Rows, error) {
	ctx, cancel := tx.db.deadline(ctx)

	result, err := tx.tx.Query(ctx, sql, args...)
	if err != nil {
		cancel()
		return nil, err
	}

	return &rows{Rows: result, cancel: cancel}, nil
}

func (tx *Tx) QueryRow(
	ctx context.Context,
	sql string,
	args ...any,
) pgx.Row {
	ctx, cancel := tx.db.deadline(ctx)

	return &row{Row: tx.tx.QueryRow(ctx, sql, args...), cancel: cancel}
}

func (tx *Tx) Commit(ctx context.Context) (err error) {
	ctx, cancel := tx.db.deadline(ctx)
	defer cancel()

	return tx.tx.Commit(ctx)
}

func (tx *Tx) End(ctx context.Context) (err error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tx.db.timeout)
	defer cancel()

	if err = tx.tx.Rollback(ctx); errors.Is(err, pgx.ErrTxClosed) {
		return nil
	}

	return err
}
