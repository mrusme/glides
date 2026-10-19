package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
)

type tx struct {
	tx *sql.Tx
}

func (d *driver) Begin(ctx context.Context) (database.DriverTx, error) {
	if d.sqldb == nil {
		return nil, errs.ErrDatabaseNotStarted
	}

	t, err := d.sqldb.BeginTx(context.WithoutCancel(ctx), nil)
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

func (t *tx) Commit(context.Context) error {
	return ConvertError(t.tx.Commit())
}

func (t *tx) Rollback(context.Context) error {
	if err := t.tx.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		return ConvertError(err)
	}

	return nil
}
