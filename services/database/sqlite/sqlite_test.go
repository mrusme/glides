package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
	"xn--gckvb8fzb.com/glides/services/database/internal/dbtest"
)

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func open(t *testing.T, opts Options) *database.Database {
	t.Helper()

	db, err := New(quiet(), opts)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func start(t *testing.T, opts Options) *database.Database {
	t.Helper()

	db := open(t, opts)
	if err := db.Startup(); err != nil {
		t.Fatalf("Startup: %v", err)
	}
	t.Cleanup(func() { db.Shutdown() })

	return db
}

func dialect(inMemory bool) dbtest.Dialect {
	return dbtest.Dialect{
		Sleep: func(time.Duration) string {
			return `WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 1000000000000) SELECT count(*) FROM c`
		},
		TableCount:     `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`,
		CreateProbe:    `CREATE TABLE dbtest_probe (id TEXT PRIMARY KEY, at TIMESTAMP, name TEXT NOT NULL UNIQUE)`,
		MigrationsPath: "migrations/sqlite",
		InMemory:       inMemory,
	}
}

func TestSuiteInMemory(t *testing.T) {
	dbtest.Run(t, func(t *testing.T, opts dbtest.Options) *database.Database {
		t.Helper()
		return open(t, Options{InMemory: true, Migrations: opts.Migrations, MigrationsPath: opts.MigrationsPath})
	}, dialect(true))
}

func TestSuiteOnAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.db")

	dbtest.Run(t, func(t *testing.T, opts dbtest.Options) *database.Database {
		t.Helper()
		return open(t, Options{Path: path, Migrations: opts.Migrations, MigrationsPath: opts.MigrationsPath})
	}, dialect(false))
}

func TestNewRejectsTheOptionCombinations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")

	for name, opts := range map[string]Options{
		"no path":                   {},
		"a path and in memory":      {Path: path, InMemory: true},
		"a path of :memory:":        {Path: ":memory:"},
		"read-only and in memory":   {InMemory: true, ReadOnly: true},
		"read-only with migrations": {Path: path, ReadOnly: true, Migrations: &dbtest.Migrations},
	} {
		if _, err := New(quiet(), opts); !errors.Is(err, errs.ErrDatabaseOptionsInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestPlaceholdersWithAGap(t *testing.T) {
	db := start(t, Options{InMemory: true})

	var a, b string
	if err := db.QueryRow(t.Context(), `SELECT $1, $3`, "x", "y", "z").Scan(&a, &b); err != nil || a != "x" || b != "z" {
		t.Errorf("$1, $3 = %q, %q, %v", a, b, err)
	}
}

func TestConcurrentWritesOnThePool(t *testing.T) {
	db := start(t, Options{Path: filepath.Join(t.TempDir(), "writes.db")})
	ctx := t.Context()

	if _, err := db.Exec(ctx, `CREATE TABLE t (n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	failures := make(chan error, 400)
	for range 8 {
		wg.Go(func() {
			for i := range 50 {
				if _, err := database.Exec(ctx, db, `INSERT INTO t (n) VALUES ($1)`, i); err != nil {
					failures <- err
				}
			}
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("a write failed: %v", err)
	}

	if n, err := database.Value[int64](ctx, db, `SELECT count(*) FROM t`); err != nil || n != 400 {
		t.Errorf("count = %d, %v, want 400", n, err)
	}
}

func TestConcurrentTransactions(t *testing.T) {
	db := start(t, Options{Path: filepath.Join(t.TempDir(), "transactions.db")})
	ctx := t.Context()

	if _, err := db.Exec(ctx, `CREATE TABLE t (n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	failures := make(chan error, 80)
	for range 4 {
		wg.Go(func() {
			for range 20 {
				if err := readThenWrite(ctx, db); err != nil {
					failures <- err
				}
			}
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("a transaction failed: %v", err)
	}

	if n, err := database.Value[int64](ctx, db, `SELECT count(*) FROM t`); err != nil || n != 80 {
		t.Errorf("count = %d, %v, want 80", n, err)
	}
}

func readThenWrite(ctx context.Context, db *database.Database) (err error) {
	tx, err := db.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.End(ctx)

	n, err := database.Value[int64](ctx, tx, `SELECT count(*) FROM t`)
	if err != nil {
		return err
	}
	if _, err = database.Exec(ctx, tx, `INSERT INTO t (n) VALUES ($1)`, n); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func TestDataSurvivesARestartInWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	db := start(t, Options{Path: path})
	ctx := t.Context()

	if _, err := db.Exec(ctx, `CREATE TABLE t (n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (n) VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if mode, err := database.Value[string](ctx, db, `PRAGMA journal_mode`); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q, %v, want wal", mode, err)
	}
	if err := db.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if err := db.Startup(); err != nil {
		t.Fatal(err)
	}
	if n, err := database.Value[int64](ctx, db, `SELECT count(*) FROM t`); err != nil || n != 1 {
		t.Errorf("count after the restart = %d, %v", n, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file isn't where Path says: %v", err)
	}
}

func TestResetBeforeStartupOnAFileWithTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reset.db")
	first := start(t, Options{Path: path})
	ctx := t.Context()

	if _, err := first.Exec(ctx, `CREATE TABLE t (n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Exec(ctx, `CREATE VIEW v AS SELECT n FROM t`); err != nil {
		t.Fatal(err)
	}
	if err := first.Shutdown(); err != nil {
		t.Fatal(err)
	}

	second := open(t, Options{Path: path})
	if err := second.Reset(); err != nil {
		t.Fatalf("Reset before Startup: %v", err)
	}
	if err := second.Startup(); err != nil {
		t.Fatal(err)
	}
	defer second.Shutdown()
	if n, err := database.Value[int64](ctx, second, `SELECT count(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`); err != nil || n != 0 {
		t.Errorf("objects after Reset = %d, %v, want 0", n, err)
	}
}

func TestInMemoryDatabasesAreSeparate(t *testing.T) {
	a := start(t, Options{InMemory: true})
	b := start(t, Options{InMemory: true})
	ctx := t.Context()

	if _, err := a.Exec(ctx, `CREATE TABLE t (n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Exec(ctx, `INSERT INTO t (n) VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Exec(ctx, `INSERT INTO t (n) VALUES (2)`); err == nil {
		t.Error("the second in-memory database shares a table with the first")
	}
	if n, err := database.Value[int64](ctx, a, `SELECT count(*) FROM t`); err != nil || n != 1 {
		t.Errorf("count in the first = %d, %v", n, err)
	}
}

func TestErrorsOfTheEngine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "errors.db")
	db := start(t, Options{Path: path})
	ctx := t.Context()

	if _, err := db.Exec(ctx, `CREATE TABLE probe (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, db, `INSERT INTO probe (id, name) VALUES ($1, $2)`, "a", "one"); err != nil {
		t.Fatal(err)
	}

	_, err := database.Exec(ctx, db, `INSERT INTO probe (id, name) VALUES ($1, $2)`, "b", "one")
	if !errors.Is(err, errs.ErrUniqueViolationOn) || !strings.HasSuffix(err.Error(), "_probe.name") {
		t.Errorf("a unique violation = %v", err)
	}
	_, err = database.Exec(ctx, db, `INSERT INTO probe (id, name) VALUES ($1, $2)`, "a", "two")
	if !errors.Is(err, errs.ErrUniqueViolationOn) || !strings.HasSuffix(err.Error(), "_probe.id") {
		t.Errorf("a primary key violation = %v", err)
	}
	if _, err = database.Exec(ctx, db, `SELEC 1`); err == nil || errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("a syntax error = %v, want the error of the engine", err)
	}

	tx, err := db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.End(ctx)
	other, err := sql.Open(driverName, "file:"+path+"?_busy_timeout=0")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	_, err = other.ExecContext(ctx, `INSERT INTO probe (id, name) VALUES ('c', 'three')`)
	if converted := ConvertError(err); err == nil || !errors.Is(converted, errs.ErrUnavailable) {
		t.Errorf("a busy database = %v, converted %v", err, converted)
	}

	missing := open(t, Options{Path: filepath.Join(t.TempDir(), "missing", "x.db")})
	if err = missing.Startup(); !errors.Is(err, errs.ErrUnavailable) {
		t.Errorf("a path whose directory doesn't exist = %v", err)
	}
}

func TestReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readonly.db")
	writer := start(t, Options{Path: path})
	ctx := t.Context()

	if _, err := writer.Exec(ctx, `CREATE TABLE t (n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec(ctx, `INSERT INTO t (n) VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Shutdown(); err != nil {
		t.Fatal(err)
	}

	reader := start(t, Options{Path: path, ReadOnly: true})
	if n, err := database.Value[int64](ctx, reader, `SELECT count(*) FROM t`); err != nil || n != 1 {
		t.Errorf("count through a read-only open = %d, %v", n, err)
	}
	if _, err := database.Exec(ctx, reader, `INSERT INTO t (n) VALUES (2)`); !errors.Is(err, errs.ErrDatabaseReadOnly) {
		t.Errorf("a write through a read-only open = %v", err)
	}
	tx, err := reader.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, tx, `INSERT INTO t (n) VALUES (2)`); !errors.Is(err, errs.ErrDatabaseReadOnly) {
		t.Errorf("a write in a transaction through a read-only open = %v", err)
	}
	if err = tx.End(ctx); err != nil {
		t.Errorf("End: %v", err)
	}
	if err = reader.Reset(); !errors.Is(err, errs.ErrDatabaseReadOnly) {
		t.Errorf("Reset through a read-only open = %v", err)
	}

	absent := filepath.Join(t.TempDir(), "absent.db")
	never := open(t, Options{Path: absent, ReadOnly: true})
	if err = never.Startup(); err == nil {
		t.Error("a read-only open created a database")
	}
	if _, err = os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the file exists after a read-only open: %v", err)
	}
}

func TestTimesAreStoredInUTCAndOrderByInstant(t *testing.T) {
	db := start(t, Options{InMemory: true})
	ctx := t.Context()

	if _, err := db.Exec(ctx, `CREATE TABLE times (at TIMESTAMP NOT NULL, ms INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	noon := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	elsewhere := time.Date(2026, 10, 19, 13, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	for _, at := range []time.Time{noon.Add(750 * time.Millisecond), noon.Add(700 * time.Millisecond), noon.Add(time.Second), elsewhere} {
		if _, err := database.Exec(ctx, db, `INSERT INTO times (at, ms) VALUES ($1, $2)`, at, at.UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}

	stored, err := database.Value[string](ctx, db, `SELECT CAST(at AS TEXT) FROM times WHERE ms = $1`, noon.Add(700*time.Millisecond).UnixMilli())
	if err != nil || stored != "2026-10-19 12:00:00.7+00:00" {
		t.Errorf("the driver stored %q, %v", stored, err)
	}
	if stored, err = database.Value[string](ctx, db, `SELECT CAST(at AS TEXT) FROM times WHERE ms = $1`, elsewhere.UnixMilli()); err != nil || stored != "2026-10-19 11:30:00+00:00" {
		t.Errorf("the driver stored a zoned time as %q, %v", stored, err)
	}

	want := []int64{elsewhere.UnixMilli(), noon.Add(700 * time.Millisecond).UnixMilli(), noon.Add(750 * time.Millisecond).UnixMilli(), noon.Add(time.Second).UnixMilli()}
	byText, err := database.Values[int64](ctx, db, `SELECT ms FROM times ORDER BY at`)
	if err != nil || !reflect.DeepEqual(byText, want) {
		t.Errorf("ORDER BY the time column = %v, %v, want the order of the instants %v", byText, err, want)
	}
	between, err := database.Value[int64](ctx, db, `SELECT count(*) FROM times WHERE at BETWEEN $1 AND $2`, elsewhere, noon.Add(750*time.Millisecond))
	if err != nil || between != 3 {
		t.Errorf("BETWEEN two times = %d, %v, want 3", between, err)
	}
	month, err := database.Value[string](ctx, db, `SELECT strftime('%Y-%m', at) FROM times WHERE ms = $1`, elsewhere.UnixMilli())
	if err != nil || month != "2026-10" {
		t.Errorf("strftime on the column = %q, %v", month, err)
	}
	back, err := database.Value[time.Time](ctx, db, `SELECT at FROM times WHERE ms = $1`, elsewhere.UnixMilli())
	if err != nil || !back.Equal(elsewhere) || back.Location() != time.UTC {
		t.Errorf("the zoned time reads back as %v in %v, %v, want the instant in UTC", back, back.Location(), err)
	}
}
