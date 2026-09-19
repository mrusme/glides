package database

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"xn--gckvb8fzb.com/glides/errs"
)

type rows struct {
	pgx.Rows
	cancel context.CancelFunc
}

func (r *rows) Close() {
	r.Rows.Close()
	r.cancel()
}

type row struct {
	pgx.Row
	cancel context.CancelFunc
}

func (r *row) Scan(dest ...any) error {
	defer r.cancel()
	return r.Row.Scan(dest...)
}

type failedRow struct {
	err error
}

func (r failedRow) Scan(...any) error {
	return r.err
}

func (db *Database) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (pgconn.CommandTag, error) {
	if db.pool == nil {
		return pgconn.CommandTag{}, errs.ErrDatabaseNotStarted
	}

	ctx, cancel := db.deadline(ctx)
	defer cancel()

	db.log.Debug("Database.Exec", "sql", sql)
	return db.pool.Exec(ctx, sql, args...)
}

func (db *Database) Query(
	ctx context.Context,
	sql string,
	args ...any,
) (pgx.Rows, error) {
	if db.pool == nil {
		return nil, errs.ErrDatabaseNotStarted
	}

	ctx, cancel := db.deadline(ctx)

	db.log.Debug("Database.Query", "sql", sql)
	result, err := db.pool.Query(ctx, sql, args...)
	if err != nil {
		cancel()
		return nil, err
	}

	return &rows{Rows: result, cancel: cancel}, nil
}

func (db *Database) QueryRow(
	ctx context.Context,
	sql string,
	args ...any,
) pgx.Row {
	if db.pool == nil {
		return failedRow{err: errs.ErrDatabaseNotStarted}
	}

	ctx, cancel := db.deadline(ctx)

	db.log.Debug("Database.QueryRow", "sql", sql)
	return &row{Row: db.pool.QueryRow(ctx, sql, args...), cancel: cancel}
}

func (db *Database) ConvertError(err error) error {
	var pgErr *pgconn.PgError

	if err == nil {
		return err
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNoRows
	}

	if errors.As(err, &pgErr) {
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

	return err
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
	var netErr net.Error

	return errors.Is(err, errs.ErrUnavailable) ||
		errors.Is(err, errs.ErrDatabaseNotStarted) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.As(err, &connectErr) ||
		errors.As(err, &netErr) ||
		pgconn.Timeout(err)
}
