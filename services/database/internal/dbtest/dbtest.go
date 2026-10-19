package dbtest

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
)

//go:embed migrations
var Migrations embed.FS

type Options struct {
	Migrations     *embed.FS
	MigrationsPath string
}

type Opener func(t *testing.T, opts Options) *database.Database

type Dialect struct {
	Sleep          func(d time.Duration) string
	TableCount     string
	CreateProbe    string
	MigrationsPath string
	InMemory       bool
}

type probe struct {
	N     int
	Twice int
}

const series string = `WITH RECURSIVE s(n) AS (
	SELECT 1 UNION ALL SELECT n + 1 FROM s WHERE n < 3
) SELECT n, n * 2 AS twice FROM s`

func Run(t *testing.T, open Opener, d Dialect) {
	t.Run("Lifecycle", func(t *testing.T) { lifecycle(t, open) })
	t.Run("Placeholders", func(t *testing.T) { placeholders(t, open) })
	t.Run("Queries", func(t *testing.T) { queries(t, open) })
	t.Run("Helpers", func(t *testing.T) { helpers(t, open) })
	t.Run("EarlyClose", func(t *testing.T) { earlyClose(t, open) })
	t.Run("Table", func(t *testing.T) { table(t, open, d) })
	t.Run("Transactions", func(t *testing.T) { transactions(t, open, d) })
	t.Run("Deadline", func(t *testing.T) { deadline(t, open, d) })
	t.Run("Migrations", func(t *testing.T) { migrations(t, open, d) })
	t.Run("Reset", func(t *testing.T) { reset(t, open, d) })
}

func start(t *testing.T, open Opener, opts Options) *database.Database {
	t.Helper()

	db := open(t, opts)
	if err := db.Startup(); err != nil {
		t.Fatalf("Startup: %v", err)
	}
	t.Cleanup(func() { db.Shutdown() })

	return db
}

func lifecycle(t *testing.T, open Opener) {
	db := open(t, Options{})
	ctx := t.Context()

	if err := db.Ping(ctx); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Ping before Startup: %v", err)
	}
	if err := db.Shutdown(); err != nil {
		t.Errorf("Shutdown before Startup: %v", err)
	}
	if err := db.Startup(); err != nil {
		t.Fatalf("Startup: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}
	if err := db.Shutdown(); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	if err := db.Shutdown(); err != nil {
		t.Errorf("a second Shutdown: %v", err)
	}
	if err := db.Ping(ctx); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Ping after Shutdown: %v", err)
	}
	if err := db.Startup(); err != nil {
		t.Fatalf("Startup after Shutdown: %v", err)
	}
	defer db.Shutdown()
	if err := db.Ping(ctx); err != nil {
		t.Errorf("Ping after the second Startup: %v", err)
	}
}

func placeholders(t *testing.T, open Opener) {
	db := start(t, open, Options{})
	ctx := t.Context()

	var a, b string
	if err := db.QueryRow(ctx, `SELECT CAST($2 AS TEXT), CAST($1 AS TEXT)`, "first", "second").Scan(&a, &b); err != nil || a != "second" || b != "first" {
		t.Errorf("$2, $1 = %q, %q, %v", a, b, err)
	}
	if err := db.QueryRow(ctx, `SELECT CAST($1 AS TEXT) || CAST($1 AS TEXT)`, "x").Scan(&a); err != nil || a != "xx" {
		t.Errorf("$1 twice = %q, %v", a, err)
	}
	if err := db.QueryRow(ctx, `SELECT CAST($1 AS TEXT), CAST($2 AS TEXT), CAST($1 AS TEXT)`, "x", "y").Scan(&a, &b, &a); err != nil || a != "x" || b != "y" {
		t.Errorf("$1, $2, $1 = %q, %q, %v", a, b, err)
	}
}

func queries(t *testing.T, open Opener) {
	db := start(t, open, Options{})
	ctx := t.Context()

	var answer int
	if err := db.QueryRow(ctx, `SELECT $1 + 1`, 41).Scan(&answer); err != nil || answer != 42 {
		t.Errorf("QueryRow = %d, %v", answer, err)
	}

	rows, err := db.Query(ctx, series)
	if err != nil {
		t.Fatal(err)
	}
	var numbers []int
	for rows.Next() {
		var n, twice int
		if err = rows.Scan(&n, &twice); err != nil {
			t.Fatal(err)
		}
		numbers = append(numbers, n)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(numbers) != 3 || numbers[2] != 3 {
		t.Errorf("Query = %v, %v", numbers, err)
	}
	if columns := rows.Columns(); len(columns) != 2 || columns[0] != "n" || columns[1] != "twice" {
		t.Errorf("Columns = %v", columns)
	}

	if _, err = db.Exec(ctx, `SELECT 1`); err != nil {
		t.Errorf("Exec: %v", err)
	}

	err = db.QueryRow(ctx, `SELECT 1 WHERE 1 = 0`).Scan(&answer)
	if !errors.Is(err, errs.ErrNoRows) {
		t.Errorf("no rows: %v", err)
	}

	gone, cancel := context.WithCancel(ctx)
	cancel()
	if err = db.QueryRow(gone, `SELECT 1`).Scan(&answer); err == nil {
		t.Error("a query ran under a context that was cancelled")
	}
	if _, err = db.Query(gone, `SELECT 1`); err == nil {
		t.Error("a query ran under a context that was cancelled")
	}
}

func earlyClose(t *testing.T, open Opener) {
	db := start(t, open, Options{})
	ctx := t.Context()

	tx, err := db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.End(ctx)

	for name, ex := range map[string]database.Executor{"database": db, "transaction": tx} {
		for round := range 40 {
			rows, err := ex.Query(ctx, series)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !rows.Next() {
				t.Fatalf("%s: no first row: %v", name, rows.Err())
			}
			var n, twice int
			if err = rows.Scan(&n, &twice); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			rows.Close()
			time.Sleep(time.Millisecond)
			if err = rows.Err(); err != nil {
				t.Fatalf("%s: Err after closing the cursor at its first row, round %d = %v, want nil",
					name, round, err)
			}
		}

		for round := range 40 {
			if _, err = database.Value[int64](ctx, ex, `SELECT n FROM (`+series+`) AS s`); err != nil {
				t.Fatalf("%s: Value, round %d = %v", name, round, err)
			}
		}
	}
}

func helpers(t *testing.T, open Opener) {
	db := start(t, open, Options{})
	ctx := t.Context()

	items, err := database.Rows[probe](ctx, db, series)
	if err != nil || len(items) != 3 || items[2].Twice != 6 {
		t.Errorf("Rows = %+v, %v", items, err)
	}

	item, err := database.Row[probe](ctx, db, series+` WHERE n = $1`, 2)
	if err != nil || item == nil || item.N != 2 || item.Twice != 4 {
		t.Errorf("Row = %+v, %v", item, err)
	}

	item, err = database.Row[probe](ctx, db, series+` WHERE n = $1`, 9)
	if !errors.Is(err, errs.ErrNoRows) || item != nil {
		t.Errorf("Row without a match = %+v, %v, want %v", item, err, errs.ErrNoRows)
	}

	none, err := database.Rows[probe](ctx, db, series+` WHERE n = $1`, 9)
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("Rows without a match = %#v, %v, want an empty slice", none, err)
	}

	total, err := database.Value[int64](ctx, db, `SELECT count(*) FROM (`+series+`) AS s`)
	if err != nil || total != 3 {
		t.Errorf("Value = %d, %v", total, err)
	}

	if _, err = database.Value[int](ctx, db, `SELECT 1 WHERE 1 = 0`); !errors.Is(err, errs.ErrNoRows) {
		t.Errorf("Value without a match = %v, want %v", err, errs.ErrNoRows)
	}

	numbers, err := database.Values[int](ctx, db, `SELECT n FROM (`+series+`) AS s`)
	if err != nil || len(numbers) != 3 || numbers[0] != 1 {
		t.Errorf("Values = %v, %v", numbers, err)
	}

	if numbers, err = database.Values[int](ctx, db, `SELECT 1 WHERE 1 = 0`); err != nil || numbers == nil || len(numbers) != 0 {
		t.Errorf("Values without a match = %#v, %v, want an empty slice", numbers, err)
	}

	if _, err = database.Exec(ctx, db, `SELECT 1`); err != nil {
		t.Errorf("Exec = %v", err)
	}

	if _, err = database.Exec(ctx, db, `SELEC 1`); err == nil || errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("Exec with a syntax error = %v, want the error of the engine", err)
	}
}

func table(t *testing.T, open Opener, d Dialect) {
	db := start(t, open, Options{})
	ctx := t.Context()

	if _, err := db.Exec(ctx, d.CreateProbe); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(context.Background(), `DROP TABLE IF EXISTS dbtest_probe`) })

	when := time.Date(2026, 10, 19, 12, 34, 56, 789000000, time.UTC)
	id, err := database.Value[uuid.UUID](ctx, db, `INSERT INTO dbtest_probe (id, at, name) VALUES ($1, $2, $3) RETURNING id`, uuid.Must(uuid.NewV7()), when, "one")
	if err != nil || id == uuid.Nil {
		t.Fatalf("RETURNING = %v, %v", id, err)
	}

	at, err := database.Value[time.Time](ctx, db, `SELECT at FROM dbtest_probe WHERE id = $1`, id)
	if err != nil || !at.Equal(when) {
		t.Errorf("the time came back as %v, %v, want %v", at, err, when)
	}

	result, err := database.Exec(ctx, db, `INSERT INTO dbtest_probe (id, at, name) VALUES ($1, $2, $3)`, uuid.Must(uuid.NewV7()), nil, "two")
	if err != nil || result.RowsAffected() != 1 {
		t.Fatalf("Exec = %v, %v", result, err)
	}
	seen, err := database.Value[sql.Null[time.Time]](ctx, db, `SELECT at FROM dbtest_probe WHERE name = $1`, "two")
	if err != nil || seen.Valid {
		t.Errorf("a NULL time came back as %v, %v", seen, err)
	}

	_, err = database.Exec(ctx, db, `INSERT INTO dbtest_probe (id, at, name) VALUES ($1, $2, $3)`, uuid.Must(uuid.NewV7()), when, "one")
	if !errors.Is(err, errs.ErrUniqueViolationOn) {
		t.Errorf("a unique violation = %v, want %v", err, errs.ErrUniqueViolationOn)
	}

	count, err := database.Value[int64](ctx, db, `SELECT count(*) FROM dbtest_probe`)
	if err != nil || count != 2 {
		t.Errorf("count = %d, %v", count, err)
	}
	result, err = database.Exec(ctx, db, `DELETE FROM dbtest_probe WHERE name = $1`, "two")
	if err != nil || result.RowsAffected() != 1 {
		t.Errorf("DELETE = %v, %v", result, err)
	}
}

func transactions(t *testing.T, open Opener, d Dialect) {
	db := start(t, open, Options{})
	ctx := t.Context()

	if _, err := db.Exec(ctx, d.CreateProbe); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(context.Background(), `DROP TABLE IF EXISTS dbtest_probe`) })

	count := func(ex database.Executor) (n int64) {
		t.Helper()
		n, err := database.Value[int64](ctx, ex, `SELECT count(*) FROM dbtest_probe`)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	tx, err := db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, tx, `INSERT INTO dbtest_probe (id, name) VALUES ($1, $2)`, uuid.Must(uuid.NewV7()), "committed"); err != nil {
		t.Fatal(err)
	}
	if count(tx) != 1 {
		t.Error("a transaction doesn't see its own row")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Errorf("Commit: %v", err)
	}
	if err = tx.End(ctx); err != nil {
		t.Errorf("End after Commit: %v", err)
	}
	if count(db) != 1 {
		t.Error("the committed row isn't there")
	}

	tx, err = db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, tx, `INSERT INTO dbtest_probe (id, name) VALUES ($1, $2)`, uuid.Must(uuid.NewV7()), "rolled back"); err != nil {
		t.Fatal(err)
	}
	gone, cancel := context.WithCancel(ctx)
	cancel()
	if err = tx.End(gone); err != nil {
		t.Errorf("End under a cancelled context: %v", err)
	}
	if count(db) != 1 {
		t.Error("End did not roll the row back")
	}
}

func deadline(t *testing.T, open Opener, d Dialect) {
	db := start(t, open, Options{})

	short, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := database.Exec(short, db, d.Sleep(2*time.Second))
	if !errors.Is(err, errs.ErrUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a statement past its deadline ended with %v", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("the statement ran for %v past a deadline of 100ms", time.Since(start))
	}
}

func migrations(t *testing.T, open Opener, d Dialect) {
	db := start(t, open, Options{Migrations: &Migrations, MigrationsPath: d.MigrationsPath})
	ctx := t.Context()
	t.Cleanup(func() {
		db.Exec(context.Background(), `DROP TABLE IF EXISTS dbtest_migrated`)
		db.Exec(context.Background(), `DROP TABLE IF EXISTS schema_migrations`)
	})

	if _, err := database.Exec(ctx, db, `INSERT INTO dbtest_migrated (n) VALUES ($1)`, 1); err != nil {
		t.Fatalf("the migration did not create the table: %v", err)
	}
	if err := db.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if err := db.Startup(); err != nil {
		t.Fatalf("Startup with the migration applied: %v", err)
	}
	n, err := database.Value[int64](ctx, db, `SELECT count(*) FROM dbtest_migrated`)
	if d.InMemory {
		if err != nil || n != 0 {
			t.Errorf("the table was not created again for a database that forgets: %d, %v", n, err)
		}
		return
	}
	if err != nil || n != 1 {
		t.Errorf("the row did not survive the second Startup: %d, %v", n, err)
	}
}

func reset(t *testing.T, open Opener, d Dialect) {
	fresh := open(t, Options{})
	if err := fresh.Reset(); err != nil {
		t.Errorf("Reset before Startup: %v", err)
	}

	db := start(t, open, Options{})
	ctx := t.Context()

	if _, err := db.Exec(ctx, d.CreateProbe); err != nil {
		t.Fatal(err)
	}
	if err := db.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	tables, err := database.Value[int64](ctx, db, d.TableCount)
	if err != nil || tables != 0 {
		t.Errorf("tables after Reset = %d, %v, want 0", tables, err)
	}
	if _, err = db.Exec(ctx, d.CreateProbe); err != nil {
		t.Errorf("the database is unusable after Reset: %v", err)
	}
	db.Exec(ctx, `DROP TABLE IF EXISTS dbtest_probe`)
}
