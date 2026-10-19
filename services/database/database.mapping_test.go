package database

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"xn--gckvb8fzb.com/glides/errs"
)

type base struct {
	ID        int
	CreatedAt time.Time
}

type inner struct {
	Deep int
}

type model struct {
	base
	inner
	Name   string `db:"label"`
	Secret string `db:"-"`
	hidden int
	Count  int
	Flags  int `db:"flags,omitempty"`
}

func TestMappingFollowsTheRulesOfPgx(t *testing.T) {
	when := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	cursor := &fakeCursor{
		columns: []string{"id", "created_at", "label", "count", "deep", "FLAGS"},
		values:  [][]any{{7, when, "seven", 3, 9, 1}},
	}
	cursor.Next()

	var got model
	if err := scanStruct(cursor, &got); err != nil {
		t.Fatal(err)
	}
	want := model{ID: 7, CreatedAt: when, Deep: 9, Name: "seven", Count: 3, Flags: 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scanStruct = %+v, want %+v", got, want)
	}
	if got.hidden != 0 || got.Secret != "" {
		t.Errorf("scanStruct filled a field that has no column: %+v", got)
	}

	cursor.columns = []string{"ID", "CREATEDAT", "label", "COUNT", "Deep", "flags"}
	cursor.pos = 0
	cursor.Next()
	got = model{}
	if err := scanStruct(cursor, &got); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("scanStruct with other cases = %+v, %v", got, err)
	}
}

func TestMappingIsStrictBothWays(t *testing.T) {
	var got model

	extra := &fakeCursor{columns: []string{"id", "created_at", "label", "count", "deep", "flags", "extra"}}
	if err := scanStruct(extra, &got); !errors.Is(err, errs.ErrRowMismatch) {
		t.Errorf("a column without a field: %v", err)
	}

	short := &fakeCursor{columns: []string{"id", "created_at", "label", "deep", "flags"}}
	if err := scanStruct(short, &got); !errors.Is(err, errs.ErrRowMismatch) {
		t.Errorf("a field without a column: %v", err)
	}

	tagged := &fakeCursor{columns: []string{"id", "created_at", "name", "count", "deep", "flags"}}
	if err := scanStruct(tagged, &got); !errors.Is(err, errs.ErrRowMismatch) {
		t.Errorf("a tag doesn't match the field name: %v", err)
	}

	var n int
	if err := scanStruct(&fakeCursor{columns: []string{"n"}}, &n); !errors.Is(err, errs.ErrRowMismatch) {
		t.Errorf("a type that isn't a struct: %v", err)
	}
}

func TestMappingIsCached(t *testing.T) {
	columns := []string{"n", "twice"}
	first, err := lookupPaths(reflect.TypeFor[probe](), columns)
	if err != nil {
		t.Fatal(err)
	}
	second, err := lookupPaths(reflect.TypeFor[probe](), columns)
	if err != nil {
		t.Fatal(err)
	}
	if &first[0] != &second[0] {
		t.Error("the second lookup computed the paths again")
	}
	if third, _ := lookupPaths(reflect.TypeFor[probe](), []string{"twice", "n"}); reflect.DeepEqual(first, third) {
		t.Error("another column order shares the paths")
	}
}
