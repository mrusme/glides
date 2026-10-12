package common

import (
	"strings"
	"testing"
)

func TestQueryOrderBy(t *testing.T) {
	tests := []struct {
		name        string
		qo          QueryOptions
		wantContain string
		wantAbsent  string
	}{
		{
			name:        "valid column ascending",
			qo:          QueryOptions{OrderBy: "position", Order: Ascending},
			wantContain: "ORDER BY position ASC",
		},
		{
			name:        "valid qualified column",
			qo:          QueryOptions{OrderBy: "vr.created_at", Order: Descending},
			wantContain: "ORDER BY vr.created_at DESC",
		},
		{
			name:        "empty order defaults to DESC",
			qo:          QueryOptions{OrderBy: "created_at"},
			wantContain: "ORDER BY created_at DESC",
		},
		{
			name:       "injection attempt is dropped",
			qo:         QueryOptions{OrderBy: "id; DROP TABLE users;--"},
			wantAbsent: "ORDER BY",
		},
		{
			name:       "subquery injection is dropped",
			qo:         QueryOptions{OrderBy: "(SELECT password FROM users)"},
			wantAbsent: "ORDER BY",
		},
		{
			name:        "malicious order direction is coerced",
			qo:          QueryOptions{OrderBy: "position", Order: QueryOrder("ASC; DROP TABLE users")},
			wantContain: "ORDER BY position DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.qo.Query("SELECT * FROM t", QueryCapabilities{})
			if tt.wantContain != "" && !strings.Contains(got, tt.wantContain) {
				t.Errorf("query %q missing %q", got, tt.wantContain)
			}
			if tt.wantAbsent != "" && strings.Contains(got, tt.wantAbsent) {
				t.Errorf("query %q must not contain %q", got, tt.wantAbsent)
			}
		})
	}
}

func TestQueryTableIdentifier(t *testing.T) {
	got := QueryOptions{}.Query(
		"SELECT * FROM t",
		QueryCapabilities{Table: "vr", HasDeleted: true},
	)
	if !strings.Contains(got, "vr.deleted_at IS NULL") {
		t.Errorf("expected qualified column, got %q", got)
	}

	got = QueryOptions{}.Query(
		"SELECT * FROM t",
		QueryCapabilities{Table: "vr;DROP TABLE users", HasDeleted: true},
	)
	if strings.Contains(got, "DROP TABLE") {
		t.Errorf("malicious table interpolated: %q", got)
	}
	if !strings.Contains(got, "deleted_at IS NULL") {
		t.Errorf("expected unqualified fallback column, got %q", got)
	}
}

func TestHasTopLevelWhere(t *testing.T) {
	for query, want := range map[string]bool{
		"SELECT * FROM users":                                              false,
		"SELECT * FROM users WHERE id = $1":                                true,
		"select * from users where id = $1":                                true,
		"SELECT * FROM users\n\t\t\tWHERE id = $1":                         true,
		"SELECT * FROM (SELECT * FROM users WHERE id = $1) AS u":           false,
		"SELECT * FROM (SELECT * FROM users WHERE id = $1) AS u WHERE u.x": true,
		"SELECT somewhere FROM places":                                     false,
		"SELECT where_clause FROM rules":                                   false,
		"SELECT * FROM notes WHERE body = 'no WHERE here'":                 true,
		"SELECT 'WHERE' AS word FROM notes":                                false,
		`SELECT "WHERE" FROM notes`:                                        false,
		"SELECT * FROM t WHERE":                                            true,
	} {
		if got := hasTopLevelWhere(query); got != want {
			t.Errorf("hasTopLevelWhere(%q) = %v, want %v", query, got, want)
		}
	}
}

func TestQueryAppendsToASubquery(t *testing.T) {
	got := QueryOptions{}.Query(
		"SELECT * FROM (SELECT * FROM users WHERE banned_at IS NULL) AS u",
		QueryCapabilities{HasDeleted: true},
	)
	want := "SELECT * FROM (SELECT * FROM users WHERE banned_at IS NULL) AS u WHERE deleted_at IS NULL"
	if got != want {
		t.Errorf("Query = %q, want %q", got, want)
	}
}

func TestQueryBuildsTheWholeStatement(t *testing.T) {
	qo := QueryOptions{Limit: 20, Page: 3, OrderBy: "created_at", Order: Descending}
	qc := QueryCapabilities{Table: "posts", HasBanned: true, HasSpammed: true, HasDeleted: true}

	got := qo.Query("SELECT * FROM posts WHERE forum_id = $1", qc)
	want := "SELECT * FROM posts WHERE forum_id = $1 AND posts.banned_at IS NULL" +
		" AND posts.spammed_at IS NULL AND posts.deleted_at IS NULL" +
		" ORDER BY created_at DESC LIMIT 20 OFFSET 40"
	if got != want {
		t.Errorf("Query = %q, want %q", got, want)
	}

	got = QueryOptions{WithBanned: true, WithSpammed: true, WithDeleted: true}.Query("SELECT * FROM posts", qc)
	if got != "SELECT * FROM posts" {
		t.Errorf("Query with every With flag = %q, want the statement untouched", got)
	}
}

func TestQueryOffsetIsNeverNegative(t *testing.T) {
	for name, qo := range map[string]QueryOptions{
		"first page with an adjustment": {Limit: 10, Page: 1, OffAdjust: 15},
		"adjustment larger than offset": {Limit: 10, Page: 2, OffAdjust: 25},
	} {
		got := qo.Query("SELECT * FROM t", QueryCapabilities{})
		if strings.Contains(got, "OFFSET -") {
			t.Errorf("%s: %q", name, got)
		}
		if !strings.HasSuffix(got, "OFFSET 0") {
			t.Errorf("%s: %q, want OFFSET 0", name, got)
		}
	}

	if got := (QueryOptions{Limit: 10, Page: 3, OffAdjust: 5}).Offset(); got != 15 {
		t.Errorf("Offset() = %d, want 15", got)
	}
	if got := (QueryOptions{Limit: 10}).Offset(); got != 0 {
		t.Errorf("Offset() without a page = %d, want 0", got)
	}
}

func TestQueryAllocatesLittle(t *testing.T) {
	qo := QueryOptions{Limit: 20, Page: 3, OrderBy: "created_at"}
	qc := QueryCapabilities{Table: "posts", HasBanned: true, HasSpammed: true, HasDeleted: true}

	allocs := testing.AllocsPerRun(100, func() {
		qo.Query("SELECT * FROM posts WHERE forum_id = $1", qc)
	})
	if allocs > 3 {
		t.Errorf("Query costs %.0f allocations", allocs)
	}
}

func BenchmarkQuery(b *testing.B) {
	qo := QueryOptions{Limit: 20, Page: 3, OrderBy: "created_at", Order: Descending}
	qc := QueryCapabilities{Table: "posts", HasBanned: true, HasSpammed: true, HasDeleted: true}
	b.ReportAllocs()

	for b.Loop() {
		qo.Query("SELECT * FROM posts WHERE forum_id = $1", qc)
	}
}
