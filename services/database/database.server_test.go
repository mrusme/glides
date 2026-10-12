package database

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func connection(t *testing.T) string {
	t.Helper()

	c := os.Getenv(envDatabase)
	if c == "" {
		t.Skipf("%s is not set", envDatabase)
	}

	return c
}

func logTo(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level}))
}

func TestStartupWithoutMigrations(t *testing.T) {
	var buf bytes.Buffer
	db, err := New(logTo(&buf, slog.LevelInfo), connection(t))
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
	db, err := New(logTo(&buf, slog.LevelDebug), connection(t))
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
	admin := connection(t)
	u, err := url.Parse(admin)
	if err != nil || !strings.HasPrefix(u.Scheme, "postgres") {
		t.Skipf("%s is not a URL, which this test needs to use a database of its own", envDatabase)
	}
	name := fmt.Sprintf("glides_reset_%d", time.Now().UnixNano())
	ctx := t.Context()

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
	db, err := New(quiet(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Startup(); err != nil {
		t.Fatalf("Startup: %v", err)
	}
	t.Cleanup(func() { db.Shutdown() })

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
