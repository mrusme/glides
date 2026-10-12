package database

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"xn--gckvb8fzb.com/glides/errs"
)

type probe struct {
	N     int
	Twice int
}

type broken struct{}

func (broken) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, io.EOF
}

func (broken) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, io.EOF
}

func (broken) QueryRow(context.Context, string, ...any) pgx.Row {
	return failedRow{err: io.EOF}
}

func TestHelpersConvertTheQueryError(t *testing.T) {
	ctx := t.Context()

	if _, err := Row[probe](ctx, broken{}, "SELECT 1"); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Row = %v, want %v", err, errs.ErrUnavailable)
	}
	if _, err := Rows[probe](ctx, broken{}, "SELECT 1"); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Rows = %v, want %v", err, errs.ErrUnavailable)
	}
	if _, err := Value[int](ctx, broken{}, "SELECT 1"); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Value = %v, want %v", err, errs.ErrUnavailable)
	}
	if _, err := Values[int](ctx, broken{}, "SELECT 1"); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Values = %v, want %v", err, errs.ErrUnavailable)
	}
	if _, err := Exec(ctx, broken{}, "SELECT 1"); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Exec = %v, want %v", err, errs.ErrUnavailable)
	}
}

func TestHelpersNeedAStartedDatabase(t *testing.T) {
	db := &Database{log: quiet()}

	if _, err := Row[probe](t.Context(), db, "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Row = %v, want %v", err, errs.ErrDatabaseNotStarted)
	}
	if _, err := Exec(t.Context(), db, "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Exec = %v, want %v", err, errs.ErrDatabaseNotStarted)
	}
}

func TestHelpersCollectRowsAndValues(t *testing.T) {
	db := open(t)
	ctx := t.Context()
	series := `SELECT n, n * 2 AS twice FROM generate_series(1, 3) AS n`

	items, err := Rows[probe](ctx, db, series)
	if err != nil || len(items) != 3 || items[2].Twice != 6 {
		t.Errorf("Rows = %+v, %v", items, err)
	}

	item, err := Row[probe](ctx, db, series+` WHERE n = $1`, 2)
	if err != nil || item == nil || item.N != 2 || item.Twice != 4 {
		t.Errorf("Row = %+v, %v", item, err)
	}

	item, err = Row[probe](ctx, db, series+` WHERE n = $1`, 9)
	if !errors.Is(err, errs.ErrNoRows) || item != nil {
		t.Errorf("Row without a match = %+v, %v, want %v", item, err, errs.ErrNoRows)
	}

	none, err := Rows[probe](ctx, db, series+` WHERE n = $1`, 9)
	if err != nil || len(none) != 0 {
		t.Errorf("Rows without a match = %+v, %v, want none", none, err)
	}

	total, err := Value[int64](ctx, db, `SELECT count(*) FROM generate_series(1, 3)`)
	if err != nil || total != 3 {
		t.Errorf("Value = %d, %v", total, err)
	}

	if _, err = Value[int](ctx, db, `SELECT 1 WHERE false`); !errors.Is(err, errs.ErrNoRows) {
		t.Errorf("Value without a match = %v, want %v", err, errs.ErrNoRows)
	}

	numbers, err := Values[int](ctx, db, `SELECT n FROM generate_series(1, 3) AS n`)
	if err != nil || len(numbers) != 3 || numbers[0] != 1 {
		t.Errorf("Values = %v, %v", numbers, err)
	}

	tag, err := Exec(ctx, db, `SELECT 1`)
	if err != nil || tag.String() != "SELECT 1" {
		t.Errorf("Exec = %q, %v", tag, err)
	}

	if _, err = Exec(ctx, db, `SELECT $1::int`, "x"); err == nil || errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Exec with a bad argument = %v, want the error of pgx", err)
	}
}

func TestHelpersRunInsideATransaction(t *testing.T) {
	db := open(t)
	ctx := t.Context()

	tx, err := db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.End(ctx)

	if _, err = Exec(ctx, tx, `CREATE TABLE glides_rows_probe (n int)`); err != nil {
		t.Fatal(err)
	}
	if _, err = Exec(ctx, tx, `INSERT INTO glides_rows_probe VALUES (1), (2)`); err != nil {
		t.Fatal(err)
	}

	count, err := Value[int64](ctx, tx, `SELECT count(*) FROM glides_rows_probe`)
	if err != nil || count != 2 {
		t.Errorf("Value in the transaction = %d, %v", count, err)
	}

	if err = tx.End(ctx); err != nil {
		t.Errorf("End: %v", err)
	}

	if _, err = Value[int64](ctx, db, `SELECT count(*) FROM glides_rows_probe`); err == nil {
		t.Error("the table survived the rollback")
	}
}
