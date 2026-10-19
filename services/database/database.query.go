package database

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"xn--gckvb8fzb.com/glides/errs"
)

type cursor struct {
	Cursor
	cancel context.CancelFunc
}

func (c *cursor) Close() {
	c.Cursor.Close()
	c.cancel()
}

type scanner struct {
	Scanner
	cancel context.CancelFunc
}

func (s *scanner) Scan(dest ...any) error {
	defer s.cancel()
	return s.Scanner.Scan(dest...)
}

type failedScanner struct {
	err error
}

func (s failedScanner) Scan(...any) error {
	return s.err
}

func (db *Database) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (Result, error) {
	if !db.started.Load() {
		return nil, errs.ErrDatabaseNotStarted
	}

	ctx, cancel := db.deadline(ctx)
	defer cancel()

	return db.driver.Exec(ctx, sql, args...)
}

func (db *Database) Query(
	ctx context.Context,
	sql string,
	args ...any,
) (Cursor, error) {
	if !db.started.Load() {
		return nil, errs.ErrDatabaseNotStarted
	}

	ctx, cancel := db.deadline(ctx)

	result, err := db.driver.Query(ctx, sql, args...)
	if err != nil {
		cancel()
		return nil, err
	}

	return &cursor{Cursor: result, cancel: cancel}, nil
}

func (db *Database) QueryRow(
	ctx context.Context,
	sql string,
	args ...any,
) Scanner {
	if !db.started.Load() {
		return failedScanner{err: errs.ErrDatabaseNotStarted}
	}

	ctx, cancel := db.deadline(ctx)

	return &scanner{Scanner: db.driver.QueryRow(ctx, sql, args...), cancel: cancel}
}

func (db *Database) ConvertError(err error) error {
	return ConvertError(err)
}

func ConvertError(err error) error {
	if err == nil || errors.Is(err, errs.ErrUnavailable) {
		return err
	}

	if isUnavailable(err) {
		return fmt.Errorf("%w: %w", errs.ErrUnavailable, err)
	}

	return err
}

func isUnavailable(err error) bool {
	var netErr net.Error

	return errors.Is(err, errs.ErrDatabaseNotStarted) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.As(err, &netErr)
}
