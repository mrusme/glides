package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func Row[T any](
	ctx context.Context,
	ex Executor,
	sql string,
	args ...any,
) (*T, error) {
	rows, err := ex.Query(ctx, sql, args...)
	if err != nil {
		return nil, ConvertError(err)
	}

	item, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[T])
	if err != nil {
		return nil, ConvertError(err)
	}

	return item, nil
}

func Rows[T any](
	ctx context.Context,
	ex Executor,
	sql string,
	args ...any,
) ([]T, error) {
	rows, err := ex.Query(ctx, sql, args...)
	if err != nil {
		return nil, ConvertError(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		return nil, ConvertError(err)
	}

	return items, nil
}

func Value[T any](
	ctx context.Context,
	ex Executor,
	sql string,
	args ...any,
) (T, error) {
	var zero T

	rows, err := ex.Query(ctx, sql, args...)
	if err != nil {
		return zero, ConvertError(err)
	}

	value, err := pgx.CollectOneRow(rows, pgx.RowTo[T])
	if err != nil {
		return zero, ConvertError(err)
	}

	return value, nil
}

func Values[T any](
	ctx context.Context,
	ex Executor,
	sql string,
	args ...any,
) ([]T, error) {
	rows, err := ex.Query(ctx, sql, args...)
	if err != nil {
		return nil, ConvertError(err)
	}

	values, err := pgx.CollectRows(rows, pgx.RowTo[T])
	if err != nil {
		return nil, ConvertError(err)
	}

	return values, nil
}

func Exec(
	ctx context.Context,
	ex Executor,
	sql string,
	args ...any,
) (pgconn.CommandTag, error) {
	tag, err := ex.Exec(ctx, sql, args...)

	return tag, ConvertError(err)
}
