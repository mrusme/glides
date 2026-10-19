package database

import (
	"context"
	"errors"
	"io"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
)

type probe struct {
	N     int
	Twice int
}

type broken struct{}

func (broken) Exec(context.Context, string, ...any) (Result, error) {
	return nil, io.EOF
}

func (broken) Query(context.Context, string, ...any) (Cursor, error) {
	return nil, io.EOF
}

func (broken) QueryRow(context.Context, string, ...any) Scanner {
	return failedScanner{err: io.EOF}
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
	db := New(nil, new(fakeDriver))

	if _, err := Row[probe](t.Context(), db, "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Row = %v, want %v", err, errs.ErrDatabaseNotStarted)
	}
	if _, err := Exec(t.Context(), db, "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Exec = %v, want %v", err, errs.ErrDatabaseNotStarted)
	}
}

func TestHelpersCollectRowsAndValues(t *testing.T) {
	driver := &fakeDriver{
		columns:  []string{"n", "twice"},
		values:   [][]any{{1, 2}, {2, 4}, {3, 6}},
		affected: 3,
	}
	db := started(t, driver)
	ctx := t.Context()

	items, err := Rows[probe](ctx, db, "series")
	if err != nil || len(items) != 3 || items[2].Twice != 6 {
		t.Errorf("Rows = %+v, %v", items, err)
	}
	if !driver.cursor.closed {
		t.Error("Rows left the cursor open")
	}

	item, err := Row[probe](ctx, db, "series")
	if err != nil || item == nil || item.N != 1 || item.Twice != 2 {
		t.Errorf("Row = %+v, %v", item, err)
	}
	if !driver.cursor.closed {
		t.Error("Row left the cursor open")
	}

	result, err := Exec(ctx, db, "update")
	if err != nil || result.RowsAffected() != 3 {
		t.Errorf("Exec = %v, %v", result, err)
	}

	driver.columns, driver.values = []string{"n"}, [][]any{{1}, {2}, {3}}
	numbers, err := Values[int](ctx, db, "series")
	if err != nil || len(numbers) != 3 || numbers[0] != 1 {
		t.Errorf("Values = %v, %v", numbers, err)
	}
	first, err := Value[int](ctx, db, "series")
	if err != nil || first != 1 {
		t.Errorf("Value = %d, %v", first, err)
	}

	driver.values = nil
	none, err := Rows[probe](ctx, db, "series")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("Rows without a match = %#v, %v, want an empty slice", none, err)
	}
	if item, err = Row[probe](ctx, db, "series"); !errors.Is(err, errs.ErrNoRows) || item != nil {
		t.Errorf("Row without a match = %+v, %v, want %v", item, err, errs.ErrNoRows)
	}
	if numbers, err = Values[int](ctx, db, "series"); err != nil || numbers == nil || len(numbers) != 0 {
		t.Errorf("Values without a match = %#v, %v, want an empty slice", numbers, err)
	}
	if _, err = Value[int](ctx, db, "series"); !errors.Is(err, errs.ErrNoRows) {
		t.Errorf("Value without a match = %v, want %v", err, errs.ErrNoRows)
	}
}

func TestHelpersReportTheCursorError(t *testing.T) {
	driver := &fakeDriver{columns: []string{"n", "twice"}, values: [][]any{{1, 2}}, cursorEr: io.EOF}
	db := started(t, driver)
	ctx := t.Context()

	if _, err := Rows[probe](ctx, db, "series"); !errors.Is(err, errs.ErrUnavailable) || !errors.Is(err, io.EOF) {
		t.Errorf("Rows = %v, want the cursor's error marked unavailable", err)
	}
	if _, err := Row[probe](ctx, db, "series"); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Row = %v, want the cursor's error marked unavailable", err)
	}
	driver.columns, driver.values = []string{"n"}, [][]any{{1}}
	if _, err := Values[int](ctx, db, "series"); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Values = %v, want the cursor's error marked unavailable", err)
	}
	if _, err := Value[int](ctx, db, "series"); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Value = %v, want the cursor's error marked unavailable", err)
	}
}
