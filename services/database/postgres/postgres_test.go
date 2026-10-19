package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
	"xn--gckvb8fzb.com/glides/services/database/internal/dbtest"
)

const envDatabase = "GLIDES_TEST_DATABASE"

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func logTo(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level}))
}

func connection(t *testing.T) string {
	t.Helper()

	c := os.Getenv(envDatabase)
	if c == "" {
		t.Skipf("%s is not set", envDatabase)
	}

	return c
}

func ownDatabase(t *testing.T, prefix string) string {
	t.Helper()

	admin := connection(t)
	u, err := url.Parse(admin)
	if err != nil || !strings.HasPrefix(u.Scheme, "postgres") {
		t.Skipf("%s is not a URL, which this test needs to use a database of its own", envDatabase)
	}
	name := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	if _, err = conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)") })

	u.Path = "/" + name

	return u.String()
}

func TestNewRejectsABrokenConnectionString(t *testing.T) {
	if _, err := New(quiet(), "not a connection string", Options{}); err == nil {
		t.Error("New accepted a connection string that doesn't parse")
	}
}

func TestStartupFailureLeavesNothingBehind(t *testing.T) {
	db, err := New(quiet(), "postgres://nobody:nothing@127.0.0.1:1/missing?sslmode=disable&connect_timeout=1", Options{})
	if err != nil {
		t.Fatal(err)
	}

	if err = db.Startup(); err == nil {
		t.Fatal("Startup succeeded without a database")
	}
	if _, err = db.Exec(t.Context(), "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Exec after a failed Startup: %v", err)
	}
	if err = db.Shutdown(); err != nil {
		t.Errorf("Shutdown after a failed Startup: %v", err)
	}
	if err = db.Shutdown(); err != nil {
		t.Errorf("a second Shutdown: %v", err)
	}
}

func TestConvertErrorMarksWhatATryLaterCanFix(t *testing.T) {
	for name, err := range map[string]error{
		"a deadline":             context.DeadlineExceeded,
		"a wrapped deadline":     fmt.Errorf("acquire: %w", context.DeadlineExceeded),
		"a closed connection":    io.ErrUnexpectedEOF,
		"a failed connect":       &pgconn.ConnectError{},
		"a network error":        &net.OpError{Op: "dial", Err: errors.New("connection refused")},
		"a database not started": errs.ErrDatabaseNotStarted,
		"an admin shutdown":      &pgconn.PgError{Code: "57P01"},
		"a full disk":            &pgconn.PgError{Code: "53100"},
		"too many connections":   &pgconn.PgError{Code: "53300"},
		"a deadlock":             &pgconn.PgError{Code: "40P01"},
		"a cancelled statement":  &pgconn.PgError{Code: "57014"},
	} {
		converted := ConvertError(err)
		if !errors.Is(converted, errs.ErrUnavailable) {
			t.Errorf("%s isn't marked unavailable: %v", name, converted)
		}
		if !errors.Is(converted, err) && !errors.As(converted, new(*pgconn.PgError)) {
			t.Errorf("%s lost its cause: %v", name, converted)
		}
	}

	for name, err := range map[string]error{
		"a syntax error":       &pgconn.PgError{Code: "42601"},
		"a check violation":    &pgconn.PgError{Code: "23514"},
		"a cancelled request":  context.Canceled,
		"an error of the code": errors.New("scan: wrong type"),
	} {
		if converted := ConvertError(err); errors.Is(converted, errs.ErrUnavailable) {
			t.Errorf("%s is marked unavailable: %v", name, converted)
		}
	}

	if got := ConvertError(pgx.ErrNoRows); got != errs.ErrNoRows {
		t.Errorf("no rows became %v", got)
	}
	unique := ConvertError(&pgconn.PgError{Code: "23505", ConstraintName: "users_username_key"})
	if !errors.Is(unique, errs.ErrUniqueViolationOn) || !strings.HasSuffix(unique.Error(), "_users_username_key") {
		t.Errorf("a unique violation became %v", unique)
	}
	if got := ConvertError(nil); got != nil {
		t.Errorf("nil became %v", got)
	}
}

func TestStartupWithoutMigrations(t *testing.T) {
	var buf bytes.Buffer
	db, err := New(logTo(&buf, slog.LevelInfo), connection(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Startup(); err != nil {
		t.Fatalf("Startup: %v", err)
	}
	t.Cleanup(func() { db.Shutdown() })

	var answer int
	if err = db.QueryRow(t.Context(), "SELECT 1").Scan(&answer); err != nil || answer != 1 {
		t.Errorf("QueryRow after Startup = %d, %v", answer, err)
	}
	if !strings.Contains(buf.String(), "migrations=none") {
		t.Errorf("Startup logged %q, want migrations=none", buf.String())
	}
}

func TestTracerLogsStatementsWithoutArguments(t *testing.T) {
	var buf bytes.Buffer
	db, err := New(logTo(&buf, slog.LevelDebug), connection(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Startup(); err != nil {
		t.Fatalf("Startup: %v", err)
	}
	t.Cleanup(func() { db.Shutdown() })

	var echo string
	if err = db.QueryRow(t.Context(), "SELECT $1::text", "glides-secret-argument").Scan(&echo); err != nil || echo != "glides-secret-argument" {
		t.Fatalf("QueryRow = %q, %v", echo, err)
	}

	log := buf.String()
	if !strings.Contains(log, "Database.Query") || !strings.Contains(log, "SELECT $1::text") {
		t.Errorf("the tracer logged %q, want a Database.Query line with the statement", log)
	}
	if strings.Contains(log, "glides-secret-argument") {
		t.Errorf("the tracer logged the arguments: %q", log)
	}
}

func TestResetDropsEveryTable(t *testing.T) {
	db, err := New(quiet(), ownDatabase(t, "glides_reset"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Startup(); err != nil {
		t.Fatalf("Startup: %v", err)
	}
	t.Cleanup(func() { db.Shutdown() })
	ctx := t.Context()

	if _, err = db.Exec(ctx, "CREATE TABLE probe (n int)"); err != nil {
		t.Fatal(err)
	}
	if err = db.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	var tables int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname = 'public'").Scan(&tables); err != nil || tables != 0 {
		t.Errorf("tables after Reset = %d, %v, want 0", tables, err)
	}
	if _, err = db.Exec(ctx, "CREATE TABLE probe (n int)"); err != nil {
		t.Errorf("the public schema is unusable after Reset: %v", err)
	}
}

func TestSuite(t *testing.T) {
	target := ownDatabase(t, "glides_dbtest")

	dbtest.Run(t, func(t *testing.T, opts dbtest.Options) *database.Database {
		t.Helper()
		db, err := New(quiet(), target, Options{Migrations: opts.Migrations, MigrationsPath: opts.MigrationsPath})
		if err != nil {
			t.Fatal(err)
		}
		return db
	}, dbtest.Dialect{
		Sleep:          func(d time.Duration) string { return fmt.Sprintf("SELECT pg_sleep(%v)", d.Seconds()) },
		TableCount:     "SELECT count(*) FROM pg_tables WHERE schemaname = 'public'",
		CreateProbe:    "CREATE TABLE dbtest_probe (id uuid PRIMARY KEY, at timestamptz, name text NOT NULL UNIQUE)",
		MigrationsPath: "migrations/postgres",
	})
}
