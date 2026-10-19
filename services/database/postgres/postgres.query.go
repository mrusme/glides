package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/database"
)

type executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type rows struct {
	pgx.Rows
	columns []string
}

func (r *rows) Columns() []string {
	if r.columns == nil {
		descriptions := r.Rows.FieldDescriptions()
		r.columns = make([]string, len(descriptions))
		for i, description := range descriptions {
			r.columns[i] = description.Name
		}
	}

	return r.columns
}

func (r *rows) Scan(dest ...any) error {
	return ConvertError(r.Rows.Scan(dest...))
}

func (r *rows) Err() error {
	return ConvertError(r.Rows.Err())
}

type row struct {
	pgx.Row
}

func (r row) Scan(dest ...any) error {
	return ConvertError(r.Row.Scan(dest...))
}

func exec(
	ctx context.Context,
	ex executor,
	sql string,
	args ...any,
) (database.Result, error) {
	tag, err := ex.Exec(ctx, sql, args...)
	if err != nil {
		return nil, ConvertError(err)
	}

	return tag, nil
}

func query(
	ctx context.Context,
	ex executor,
	sql string,
	args ...any,
) (database.Cursor, error) {
	result, err := ex.Query(ctx, sql, args...)
	if err != nil {
		return nil, ConvertError(err)
	}

	return &rows{Rows: result}, nil
}

func queryRow(
	ctx context.Context,
	ex executor,
	sql string,
	args ...any,
) database.Scanner {
	return row{Row: ex.QueryRow(ctx, sql, args...)}
}

func (d *driver) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (database.Result, error) {
	if d.pool == nil {
		return nil, errs.ErrDatabaseNotStarted
	}

	return exec(ctx, d.pool, sql, args...)
}

func (d *driver) Query(
	ctx context.Context,
	sql string,
	args ...any,
) (database.Cursor, error) {
	if d.pool == nil {
		return nil, errs.ErrDatabaseNotStarted
	}

	return query(ctx, d.pool, sql, args...)
}

func (d *driver) QueryRow(
	ctx context.Context,
	sql string,
	args ...any,
) database.Scanner {
	if d.pool == nil {
		return row{Row: failedRow{err: errs.ErrDatabaseNotStarted}}
	}

	return queryRow(ctx, d.pool, sql, args...)
}

type failedRow struct {
	err error
}

func (r failedRow) Scan(...any) error {
	return r.err
}

func ConvertError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNoRows
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch {
		case pgErr.Code == pgerrcode.UniqueViolation:
			return fmt.Errorf("%w_%s", errs.ErrUniqueViolationOn, pgErr.ConstraintName)
		case isUnavailableCode(pgErr.Code):
			return fmt.Errorf("%w: %w", errs.ErrUnavailable, err)
		default:
			return err
		}
	}

	if isUnavailable(err) {
		return fmt.Errorf("%w: %w", errs.ErrUnavailable, err)
	}

	return database.ConvertError(err)
}

func isUnavailableCode(code string) bool {
	return pgerrcode.IsConnectionException(code) ||
		pgerrcode.IsInsufficientResources(code) ||
		pgerrcode.IsOperatorIntervention(code) ||
		pgerrcode.IsTransactionRollback(code) ||
		code == pgerrcode.LockNotAvailable
}

func isUnavailable(err error) bool {
	var connectErr *pgconn.ConnectError

	return errors.As(err, &connectErr) || pgconn.Timeout(err)
}
