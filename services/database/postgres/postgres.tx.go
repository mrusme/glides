package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
)

type tx struct {
	tx pgx.Tx
}

func (d *driver) Begin(ctx context.Context) (database.DriverTx, error) {
	if d.pool == nil {
		return nil, errs.ErrDatabaseNotStarted
	}

	t, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, ConvertError(err)
	}

	return &tx{tx: t}, nil
}

func (t *tx) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (database.Result, error) {
	return exec(ctx, t.tx, sql, args...)
}

func (t *tx) Query(
	ctx context.Context,
	sql string,
	args ...any,
) (database.Cursor, error) {
	return query(ctx, t.tx, sql, args...)
}

func (t *tx) QueryRow(
	ctx context.Context,
	sql string,
	args ...any,
) database.Scanner {
	return queryRow(ctx, t.tx, sql, args...)
}

func (t *tx) Commit(ctx context.Context) error {
	return ConvertError(t.tx.Commit(ctx))
}

func (t *tx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); !errors.Is(err, pgx.ErrTxClosed) {
		return ConvertError(err)
	}

	return nil
}
