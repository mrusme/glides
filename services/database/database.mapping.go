package database

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"xn--gckvb8fzb.com/glides/errs"
)

const structTag string = "db"

type mappingKey struct {
	typ     reflect.Type
	columns string
}

var mappings sync.Map

func scanStruct(rows Cursor, dest any) error {
	value := reflect.ValueOf(dest).Elem()

	paths, err := lookupPaths(value.Type(), rows.Columns())
	if err != nil {
		return err
	}

	targets := make([]any, len(paths))
	for i, path := range paths {
		targets[i] = value.FieldByIndex(path).Addr().Interface()
	}

	return rows.Scan(targets...)
}

func lookupPaths(typ reflect.Type, columns []string) ([][]int, error) {
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: %s is not a struct", errs.ErrRowMismatch, typ)
	}

	key := mappingKey{typ: typ, columns: strings.Join(columns, "\x00")}
	if cached, ok := mappings.Load(key); ok {
		return cached.([][]int), nil
	}

	paths := make([][]int, len(columns))
	missing := matchFields(typ, columns, paths, nil)

	for i, path := range paths {
		if path == nil {
			return nil, fmt.Errorf("%w: no field for column %s", errs.ErrRowMismatch, columns[i])
		}
	}
	if missing != "" {
		return nil, fmt.Errorf("%w: no column for field %s", errs.ErrRowMismatch, missing)
	}

	cached, _ := mappings.LoadOrStore(key, paths)

	return cached.([][]int), nil
}

func matchFields(
	typ reflect.Type,
	columns []string,
	paths [][]int,
	stack []int,
) (missing string) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}

		path := make([]int, len(stack)+1)
		copy(path, stack)
		path[len(stack)] = i

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if sub := matchFields(field.Type, columns, paths, path); missing == "" {
				missing = sub
			}
			continue
		}

		tag, tagged := field.Tag.Lookup(structTag)
		if tagged {
			tag, _, _ = strings.Cut(tag, ",")
		}
		if tag == "-" {
			continue
		}

		name := field.Name
		if tagged {
			name = tag
		}

		pos := columnIndex(columns, name, !tagged)
		if pos == -1 {
			if missing == "" {
				missing = name
			}
			continue
		}

		paths[pos] = path
	}

	return missing
}

func columnIndex(columns []string, name string, normalize bool) int {
	if normalize {
		name = strings.ReplaceAll(name, "_", "")
		for i, column := range columns {
			if strings.EqualFold(strings.ReplaceAll(column, "_", ""), name) {
				return i
			}
		}
		return -1
	}

	for i, column := range columns {
		if column == name {
			return i
		}
	}
	for i, column := range columns {
		if strings.EqualFold(column, name) {
			return i
		}
	}

	return -1
}
