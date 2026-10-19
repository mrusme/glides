package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	engine "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
)

type executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type rows struct {
	*sql.Rows
	columns []string
}

func (r *rows) Columns() []string {
	return r.columns
}

func (r *rows) Scan(dest ...any) error {
	return ConvertError(r.Rows.Scan(dest...))
}

func (r *rows) Err() error {
	return ConvertError(r.Rows.Err())
}

func (r *rows) Close() {
	r.Rows.Close()
}

type row struct {
	*sql.Row
}

func (r row) Scan(dest ...any) error {
	return ConvertError(r.Row.Scan(dest...))
}

type failedRow struct {
	err error
}

func (r failedRow) Scan(...any) error {
	return r.err
}

type result int64

func (r result) RowsAffected() int64 {
	return int64(r)
}

func exec(
	ctx context.Context,
	ex executor,
	query string,
	args ...any,
) (database.Result, error) {
	res, err := ex.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, ConvertError(err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return nil, ConvertError(err)
	}

	return result(affected), nil
}

func query(
	ctx context.Context,
	ex executor,
	query string,
	args ...any,
) (database.Cursor, error) {
	res, err := ex.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, ConvertError(err)
	}

	columns, err := res.Columns()
	if err != nil {
		res.Close()
		return nil, ConvertError(err)
	}

	return &rows{Rows: res, columns: columns}, nil
}

func queryRow(
	ctx context.Context,
	ex executor,
	query string,
	args ...any,
) database.Scanner {
	return row{Row: ex.QueryRowContext(ctx, query, args...)}
}

func (d *driver) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (database.Result, error) {
	if d.sqldb == nil {
		return nil, errs.ErrDatabaseNotStarted
	}

	return exec(ctx, d.sqldb, sql, args...)
}

func (d *driver) Query(
	ctx context.Context,
	sql string,
	args ...any,
) (database.Cursor, error) {
	if d.sqldb == nil {
		return nil, errs.ErrDatabaseNotStarted
	}

	return query(ctx, d.sqldb, sql, args...)
}

func (d *driver) QueryRow(
	ctx context.Context,
	sql string,
	args ...any,
) database.Scanner {
	if d.sqldb == nil {
		return failedRow{err: errs.ErrDatabaseNotStarted}
	}

	return queryRow(ctx, d.sqldb, sql, args...)
}

func ConvertError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return errs.ErrNoRows
	}

	if serr, ok := errors.AsType[*engine.Error](err); ok {
		code := serr.Code()
		switch {
		case code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return fmt.Errorf("%w_%s", errs.ErrUniqueViolationOn, constraint(serr.Error()))
		case code&0xff == sqlite3.SQLITE_READONLY:
			return fmt.Errorf("%w: %w", errs.ErrDatabaseReadOnly, err)
		case isUnavailableCode(code & 0xff):
			return fmt.Errorf("%w: %w", errs.ErrUnavailable, err)
		default:
			return err
		}
	}

	return database.ConvertError(err)
}

func isUnavailableCode(code int) bool {
	switch code {
	case sqlite3.SQLITE_BUSY,
		sqlite3.SQLITE_LOCKED,
		sqlite3.SQLITE_NOMEM,
		sqlite3.SQLITE_IOERR,
		sqlite3.SQLITE_FULL,
		sqlite3.SQLITE_CANTOPEN,
		sqlite3.SQLITE_PROTOCOL:
		return true
	}

	return false
}

func constraint(message string) string {
	const marker string = "constraint failed: "

	i := strings.LastIndex(message, marker)
	if i < 0 {
		return ""
	}

	rest := message[i+len(marker):]
	if j := strings.LastIndex(rest, " ("); j >= 0 {
		rest = rest[:j]
	}

	return rest
}
