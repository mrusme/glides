package database

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"xn--gckvb8fzb.com/glides/errs"
)

const envDatabase = "GLIDES_TEST_DATABASE"

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDeadline(t *testing.T) {
	db := &Database{timeout: time.Minute}

	ctx, cancel := db.deadline(context.Background())
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > time.Minute || time.Until(deadline) < 50*time.Second {
		t.Errorf("a context without a deadline got %v, %v", deadline, ok)
	}
	cancel()
	if ctx.Err() == nil {
		t.Error("cancel did not end the context that deadline created")
	}

	parent, stop := context.WithTimeout(context.Background(), time.Hour)
	defer stop()
	ctx, cancel = db.deadline(parent)
	if ctx != parent {
		t.Error("a context that has a deadline was wrapped")
	}
	cancel()
	if parent.Err() != nil {
		t.Error("cancel ended a context that belongs to the caller")
	}
}

type fakeRows struct {
	pgx.Rows
	closed bool
}

func (r *fakeRows) Close() { r.closed = true }

type fakeRow struct{ scanned bool }

func (r *fakeRow) Scan(...any) error {
	r.scanned = true
	return nil
}

func TestResultsReleaseTheirContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inner := new(fakeRows)
	result := &rows{Rows: inner, cancel: cancel}
	if ctx.Err() != nil {
		t.Fatal("the context ended before the rows were closed")
	}
	result.Close()
	if !inner.closed || ctx.Err() == nil {
		t.Errorf("Close: closed=%v, context=%v", inner.closed, ctx.Err())
	}

	ctx, cancel = context.WithCancel(context.Background())
	single := new(fakeRow)
	if err := (&row{Row: single, cancel: cancel}).Scan(); err != nil {
		t.Fatal(err)
	}
	if !single.scanned || ctx.Err() == nil {
		t.Errorf("Scan: scanned=%v, context=%v", single.scanned, ctx.Err())
	}
}

func TestStartupFailureLeavesNothingBehind(t *testing.T) {
	db, err := New(quiet(), "postgres://nobody:nothing@127.0.0.1:1/missing?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}

	if err = db.Startup(); err == nil {
		t.Fatal("Startup succeeded without a database")
	}
	if _, err = db.Exec(t.Context(), "SELECT 1"); err == nil {
		t.Error("Exec succeeded after a failed Startup")
	}
	if err = db.Shutdown(); err != nil {
		t.Errorf("Shutdown after a failed Startup: %v", err)
	}
	if err = db.Shutdown(); err != nil {
		t.Errorf("a second Shutdown: %v", err)
	}

	if _, err = New(quiet(), "not a connection string"); err == nil {
		t.Error("New accepted a connection string that doesn't parse")
	}
}

func TestUseBeforeStartupAndAfterShutdown(t *testing.T) {
	never, err := New(quiet(), "postgres://nobody@127.0.0.1:1/missing")
	if err != nil {
		t.Fatal(err)
	}

	var n int
	if _, err = never.Exec(t.Context(), "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Exec before Startup: %v", err)
	}
	if _, err = never.Query(t.Context(), "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Query before Startup: %v", err)
	}
	if err = never.QueryRow(t.Context(), "SELECT 1").Scan(&n); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("QueryRow before Startup: %v", err)
	}
	if _, err = never.Tx(t.Context()); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Tx before Startup: %v", err)
	}
	if err = never.Shutdown(); err != nil {
		t.Errorf("Shutdown before Startup: %v", err)
	}

	db := open(t)
	if err = db.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(t.Context(), "SELECT 1").Scan(&n); err == nil {
		t.Error("QueryRow succeeded after Shutdown")
	}
	if _, err = db.Query(t.Context(), "SELECT 1"); err == nil {
		t.Error("Query succeeded after Shutdown")
	}
}

func open(t *testing.T) *Database {
	t.Helper()

	connection := os.Getenv(envDatabase)
	if connection == "" {
		t.Skipf("%s is not set", envDatabase)
	}

	db, err := New(quiet(), connection)
	if err != nil {
		t.Fatal(err)
	}
	if db.pool, err = newPool(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Shutdown() })

	return db
}

func TestQueries(t *testing.T) {
	db := open(t)
	ctx := t.Context()

	var answer int
	if err := db.QueryRow(ctx, "SELECT $1::int + 1", 41).Scan(&answer); err != nil || answer != 42 {
		t.Errorf("QueryRow = %d, %v", answer, err)
	}

	result, err := db.Query(ctx, "SELECT generate_series(1, 3)")
	if err != nil {
		t.Fatal(err)
	}
	numbers, err := pgx.CollectRows(result, pgx.RowTo[int])
	if err != nil || len(numbers) != 3 {
		t.Errorf("Query = %v, %v", numbers, err)
	}

	if _, err = db.Exec(ctx, "SELECT 1"); err != nil {
		t.Errorf("Exec: %v", err)
	}

	err = db.QueryRow(ctx, "SELECT 1 WHERE false").Scan(&answer)
	if !errors.Is(db.ConvertError(err), errs.ErrNoRows) {
		t.Errorf("no rows: %v", err)
	}

	gone, cancel := context.WithCancel(ctx)
	cancel()
	if err = db.QueryRow(gone, "SELECT 1").Scan(&answer); err == nil {
		t.Error("a query ran under a context that was cancelled")
	}
	if _, err = db.Query(gone, "SELECT 1"); err == nil {
		t.Error("a query ran under a context that was cancelled")
	}

	short := &Database{log: db.log, pool: db.pool, timeout: 50 * time.Millisecond}
	if _, err = short.Exec(context.Background(), "SELECT pg_sleep(2)"); err == nil {
		t.Error("the default timeout did not end a statement that runs for 2s")
	}
	start := time.Now()
	long, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if _, err = short.Exec(long, "SELECT pg_sleep(0.2)"); err != nil {
		t.Errorf("the default timeout was put on a context that has a deadline: %v after %v", err, time.Since(start))
	}
}

func TestTransactions(t *testing.T) {
	db := open(t)
	ctx := t.Context()

	count := func(tx *Tx) (n int) {
		t.Helper()
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM pg_class WHERE relname = 'glides_tx_probe'").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	tx, err := db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE glides_tx_probe (n int)"); err != nil {
		t.Fatal(err)
	}
	if count(tx) != 1 {
		t.Error("a transaction doesn't see its own table")
	}
	if err = tx.End(ctx); err != nil {
		t.Errorf("End: %v", err)
	}

	tx, err = db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count(tx) != 0 {
		t.Error("End did not roll the table back")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Errorf("Commit: %v", err)
	}
	if err = tx.End(ctx); err != nil {
		t.Errorf("End after Commit: %v", err)
	}

	tx, err = db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gone, cancel := context.WithCancel(ctx)
	cancel()
	if err = tx.End(gone); err != nil {
		t.Errorf("End under a cancelled context did not roll back: %v", err)
	}
}
