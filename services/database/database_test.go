package database

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"xn--gckvb8fzb.com/glides/errs"
)

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeResult int64

func (r fakeResult) RowsAffected() int64 {
	return int64(r)
}

type fakeCursor struct {
	columns []string
	values  [][]any
	pos     int
	closed  bool
	err     error
}

func (c *fakeCursor) Next() bool {
	if c.closed || c.pos >= len(c.values) {
		return false
	}
	c.pos++
	return true
}

func (c *fakeCursor) Scan(dest ...any) error {
	return assign(c.values[c.pos-1], dest)
}

func (c *fakeCursor) Columns() []string {
	return c.columns
}

func (c *fakeCursor) Err() error {
	return c.err
}

func (c *fakeCursor) Close() {
	c.closed = true
}

type fakeScanner struct {
	values []any
	err    error
}

func (s fakeScanner) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	return assign(s.values, dest)
}

func assign(values []any, dest []any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("scan: %d values for %d destinations", len(values), len(dest))
	}
	for i, value := range values {
		target := reflect.ValueOf(dest[i]).Elem()
		if value == nil {
			target.Set(reflect.Zero(target.Type()))
			continue
		}
		target.Set(reflect.ValueOf(value).Convert(target.Type()))
	}
	return nil
}

type fakeTx struct {
	driver           *fakeDriver
	commits          int
	rollbacks        int
	commitDeadline   bool
	rollbackDeadline bool
	rollbackErr      error
}

func (t *fakeTx) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	return t.driver.Exec(ctx, sql, args...)
}

func (t *fakeTx) Query(ctx context.Context, sql string, args ...any) (Cursor, error) {
	return t.driver.Query(ctx, sql, args...)
}

func (t *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) Scanner {
	return t.driver.QueryRow(ctx, sql, args...)
}

func (t *fakeTx) Commit(ctx context.Context) error {
	t.commits++
	_, t.commitDeadline = ctx.Deadline()
	return nil
}

func (t *fakeTx) Rollback(ctx context.Context) error {
	t.rollbacks++
	_, t.rollbackDeadline = ctx.Deadline()
	t.rollbackErr = ctx.Err()
	return nil
}

type fakeDriver struct {
	mu       sync.Mutex
	openErr  error
	opened   bool
	closes   int
	resets   int
	lastCtx  context.Context
	columns  []string
	values   [][]any
	cursor   *fakeCursor
	cursorEr error
	execErr  error
	queryErr error
	rowErr   error
	affected int64
	tx       *fakeTx
}

func (d *fakeDriver) remember(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastCtx = ctx
}

func (d *fakeDriver) last() context.Context {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastCtx
}

func (d *fakeDriver) Open(context.Context) error {
	if d.openErr != nil {
		return d.openErr
	}
	d.opened = true
	return nil
}

func (d *fakeDriver) Close() error {
	d.opened = false
	d.closes++
	return nil
}

func (d *fakeDriver) Ping(ctx context.Context) error {
	d.remember(ctx)
	return nil
}

func (d *fakeDriver) Reset(context.Context) error {
	d.resets++
	return nil
}

func (d *fakeDriver) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	d.remember(ctx)
	if d.execErr != nil {
		return nil, d.execErr
	}
	return fakeResult(d.affected), nil
}

func (d *fakeDriver) Query(ctx context.Context, sql string, args ...any) (Cursor, error) {
	d.remember(ctx)
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	d.cursor = &fakeCursor{columns: d.columns, values: d.values, err: d.cursorEr}
	return d.cursor, nil
}

func (d *fakeDriver) QueryRow(ctx context.Context, sql string, args ...any) Scanner {
	d.remember(ctx)
	if d.rowErr != nil {
		return fakeScanner{err: d.rowErr}
	}
	if len(d.values) == 0 {
		return fakeScanner{err: errs.ErrNoRows}
	}
	return fakeScanner{values: d.values[0]}
}

func (d *fakeDriver) Begin(ctx context.Context) (DriverTx, error) {
	d.remember(ctx)
	d.tx = &fakeTx{driver: d}
	return d.tx, nil
}

func started(t *testing.T, driver *fakeDriver) *Database {
	t.Helper()

	db := New(quiet(), driver)
	if err := db.Startup(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Shutdown() })

	return db
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

func TestMethodsNeedAStartedDatabase(t *testing.T) {
	driver := new(fakeDriver)
	db := New(nil, driver)
	ctx := t.Context()
	var n int

	if _, err := db.Exec(ctx, "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Exec before Startup: %v", err)
	}
	if _, err := db.Query(ctx, "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Query before Startup: %v", err)
	}
	if err := db.QueryRow(ctx, "SELECT 1").Scan(&n); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("QueryRow before Startup: %v", err)
	}
	if _, err := db.Tx(ctx); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Tx before Startup: %v", err)
	}
	if err := db.Ping(ctx); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Ping before Startup: %v", err)
	}
	if err := db.Shutdown(); err != nil || driver.closes != 0 {
		t.Errorf("Shutdown before Startup: %v, closes=%d", err, driver.closes)
	}

	if err := db.Startup(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "SELECT 1"); err != nil {
		t.Errorf("Exec after Startup: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		t.Errorf("Ping after Startup: %v", err)
	}

	if err := db.Shutdown(); err != nil || driver.closes != 1 {
		t.Errorf("Shutdown: %v, closes=%d", err, driver.closes)
	}
	if err := db.Shutdown(); err != nil || driver.closes != 1 {
		t.Errorf("a second Shutdown: %v, closes=%d", err, driver.closes)
	}
	if _, err := db.Exec(ctx, "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Exec after Shutdown: %v", err)
	}
	if err := db.QueryRow(ctx, "SELECT 1").Scan(&n); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("QueryRow after Shutdown: %v", err)
	}

	broken := &fakeDriver{openErr: io.EOF}
	never := New(nil, broken)
	if err := never.Startup(); !errors.Is(err, io.EOF) {
		t.Errorf("Startup with a driver that can't open: %v", err)
	}
	if _, err := never.Exec(ctx, "SELECT 1"); !errors.Is(err, errs.ErrDatabaseNotStarted) {
		t.Errorf("Exec after a failed Startup: %v", err)
	}
	if err := never.Shutdown(); err != nil || broken.closes != 0 {
		t.Errorf("Shutdown after a failed Startup: %v, closes=%d", err, broken.closes)
	}
}

func TestDefaultTimeoutReachesTheDriver(t *testing.T) {
	driver := new(fakeDriver)
	db := started(t, driver)

	if _, err := db.Exec(context.Background(), "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	deadline, ok := driver.last().Deadline()
	if !ok || time.Until(deadline) > DefaultTimeout || time.Until(deadline) < DefaultTimeout-time.Second {
		t.Errorf("the driver got a deadline of %v, %v, want %v from now", deadline, ok, DefaultTimeout)
	}

	own, stop := context.WithTimeout(context.Background(), time.Hour)
	defer stop()
	if _, err := db.Exec(own, "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if driver.last() != own {
		t.Error("a context with a deadline of its own was wrapped")
	}
}

func TestResultsReleaseTheirContext(t *testing.T) {
	driver := &fakeDriver{columns: []string{"n"}, values: [][]any{{1}}}
	db := started(t, driver)
	ctx := context.Background()

	if _, err := db.Exec(ctx, "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if driver.last().Err() == nil {
		t.Error("Exec kept its context alive after it returned")
	}

	rows, err := db.Query(ctx, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if driver.last().Err() != nil {
		t.Error("the context ended before the cursor was closed")
	}
	rows.Close()
	if !driver.cursor.closed || driver.last().Err() == nil {
		t.Errorf("Close: closed=%v, context=%v", driver.cursor.closed, driver.last().Err())
	}

	var n int
	single := db.QueryRow(ctx, "SELECT 1")
	if driver.last().Err() != nil {
		t.Error("the context ended before the scanner scanned")
	}
	if err = single.Scan(&n); err != nil || n != 1 {
		t.Fatalf("Scan = %d, %v", n, err)
	}
	if driver.last().Err() == nil {
		t.Error("Scan kept its context alive")
	}

	tx, err := db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if driver.last().Err() == nil {
		t.Error("Tx kept the context of Begin alive")
	}
	rows, err = tx.Query(ctx, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if driver.last().Err() != nil {
		t.Error("the context of a transaction's query ended before Close")
	}
	rows.Close()
	if driver.last().Err() == nil {
		t.Error("Close of a transaction's cursor kept its context alive")
	}
}

func TestConvertErrorMarksWhatATryLaterCanFix(t *testing.T) {
	db := New(nil, new(fakeDriver))

	for name, err := range map[string]error{
		"a deadline":             context.DeadlineExceeded,
		"a wrapped deadline":     fmt.Errorf("acquire: %w", context.DeadlineExceeded),
		"a closed connection":    io.ErrUnexpectedEOF,
		"an end of file":         io.EOF,
		"a network error":        &net.OpError{Op: "dial", Err: errors.New("connection refused")},
		"a database not started": errs.ErrDatabaseNotStarted,
	} {
		converted := db.ConvertError(err)
		if !errors.Is(converted, errs.ErrUnavailable) {
			t.Errorf("%s isn't marked unavailable: %v", name, converted)
		}
		if !errors.Is(converted, err) {
			t.Errorf("%s lost its cause: %v", name, converted)
		}
		twice := ConvertError(converted)
		if twice != converted || strings.Count(twice.Error(), errs.ErrUnavailable.Error()) != 1 {
			t.Errorf("%s converted twice became %v", name, twice)
		}
	}

	for name, err := range map[string]error{
		"a cancelled request":  context.Canceled,
		"an error of the code": errors.New("scan: wrong type"),
		"no rows":              errs.ErrNoRows,
	} {
		if converted := ConvertError(err); converted != err {
			t.Errorf("%s became %v", name, converted)
		}
	}

	if got := ConvertError(nil); got != nil {
		t.Errorf("nil became %v", got)
	}
}

func TestTransactionsDetachEndFromTheCancellation(t *testing.T) {
	driver := new(fakeDriver)
	db := started(t, driver)
	ctx := t.Context()

	tx, err := db.Tx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil || driver.tx.commits != 1 {
		t.Errorf("Commit: %v, commits=%d", err, driver.tx.commits)
	}
	if !driver.tx.commitDeadline {
		t.Error("Commit ran without a deadline")
	}

	gone, cancel := context.WithCancel(ctx)
	cancel()
	if err = tx.End(gone); err != nil || driver.tx.rollbacks != 1 {
		t.Errorf("End: %v, rollbacks=%d", err, driver.tx.rollbacks)
	}
	if driver.tx.rollbackErr != nil {
		t.Error("End ran the rollback under the cancelled context")
	}
	if !driver.tx.rollbackDeadline {
		t.Error("End ran the rollback without a deadline")
	}
}

func TestResetRunsStartedOrNot(t *testing.T) {
	var buf bytes.Buffer
	driver := new(fakeDriver)
	db := New(slog.New(slog.NewTextHandler(&buf, nil)), driver)

	if err := db.Reset(); err != nil || driver.resets != 1 {
		t.Errorf("Reset before Startup: %v, resets=%d", err, driver.resets)
	}
	if err := db.Startup(); err != nil {
		t.Fatal(err)
	}
	defer db.Shutdown()
	if err := db.Reset(); err != nil || driver.resets != 2 {
		t.Errorf("Reset after Startup: %v, resets=%d", err, driver.resets)
	}
	if log := buf.String(); !strings.Contains(log, "level=WARN") || !strings.Contains(log, "Database.Reset") || !strings.Contains(log, "status=ok") {
		t.Errorf("Reset logged %q", log)
	}
}
