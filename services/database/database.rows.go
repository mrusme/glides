package database

import (
	"context"

	"xn--gckvb8fzb.com/glides/errs"
)

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
	defer rows.Close()

	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return nil, ConvertError(err)
		}
		return nil, errs.ErrNoRows
	}

	item := new(T)
	if err = scanStruct(rows, item); err != nil {
		return nil, ConvertError(err)
	}

	rows.Close()
	if err = rows.Err(); err != nil {
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
	defer rows.Close()

	items := []T{}
	for rows.Next() {
		var item T
		if err = scanStruct(rows, &item); err != nil {
			return nil, ConvertError(err)
		}
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
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
	defer rows.Close()

	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return zero, ConvertError(err)
		}
		return zero, errs.ErrNoRows
	}

	var value T
	if err = rows.Scan(&value); err != nil {
		return zero, ConvertError(err)
	}

	rows.Close()
	if err = rows.Err(); err != nil {
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
	defer rows.Close()

	values := []T{}
	for rows.Next() {
		var value T
		if err = rows.Scan(&value); err != nil {
			return nil, ConvertError(err)
		}
		values = append(values, value)
	}

	if err = rows.Err(); err != nil {
		return nil, ConvertError(err)
	}

	return values, nil
}

func Exec(
	ctx context.Context,
	ex Executor,
	sql string,
	args ...any,
) (Result, error) {
	result, err := ex.Exec(ctx, sql, args...)

	return result, ConvertError(err)
}
