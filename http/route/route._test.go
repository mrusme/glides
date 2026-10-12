package route

import "testing"

func testTable() Table {
	return Table{
		"Root":                   {Hierarchy: []string{"root"}},
		"Categories":             {Hierarchy: []string{"root", "_:categories"}},
		"CategoriesForums":       {Hierarchy: []string{"root", "_:categories", ":forums"}},
		"CategoriesForumsTopics": {Hierarchy: []string{"root", "_:categories", ":forums", ":topics"}},
		"Admin":                  {Hierarchy: []string{"root", "admin"}},
		"AdminUsers":             {Hierarchy: []string{"root", "admin", "users"}},
		"Session":                {Hierarchy: []string{"root", "session"}, NoBreadcrumb: true},
		"SessionConfirm":         {Hierarchy: []string{"root", "session", "confirm"}},
	}
}

func TestRouteFill(t *testing.T) {
	Use(testTable(), "Example Board")

	r := For("CategoriesForumsTopics")
	fr := r.Fill(
		map[string]string{
			"categories": "hosting",
			"forums":     "aws",
			"topics":     "evil",
		},
	)

	if fr.AsURL() != "_hosting/aws/evil" {
		t.Errorf("Fill() incorrect: %s", fr.AsURL())
	}
}

func TestPreparedRoutesMatchTheComputedOnes(t *testing.T) {
	table := testTable()
	Use(table, "Example Board")

	for id, raw := range table {
		got := For(id)
		if !got.prepared {
			t.Errorf("%s: For returned an unprepared route", id)
		}
		if got.AsURL() != raw.AsURL() || got.AsID() != raw.AsID() ||
			got.AsTitle() != raw.AsTitle() || got.Parent() != raw.Parent() {
			t.Errorf("%s: prepared %q %q %q %q, computed %q %q %q %q", id,
				got.AsURL(), got.AsID(), got.AsTitle(), got.Parent(),
				raw.AsURL(), raw.AsID(), raw.AsTitle(), raw.Parent())
		}
	}
}

func TestParentsAndTitles(t *testing.T) {
	Use(testTable(), "Example Board")

	if got := For("AdminUsers").Parent(); got != "Admin" {
		t.Errorf("Parent() = %q, want Admin", got)
	}
	if got := For("AdminUsers").ParentRoute().AsTitle(); got != "root_admin_title" {
		t.Errorf("ParentRoute().AsTitle() = %q", got)
	}
	if got := For("CategoriesForums").Parent(); got != "Categories" {
		t.Errorf("Parent() of a parametrized route = %q, want Categories", got)
	}
	if got := For("CategoriesForums").AsTitle(); got != "" {
		t.Errorf("AsTitle() of a parametrized route = %q, want empty", got)
	}
	if got := For("Root").Parent(); got != "" {
		t.Errorf("Parent() of the root = %q, want empty", got)
	}
	if got := For("Nothing").Len(); got != 0 {
		t.Errorf("an unknown id resolved to a route of %d segments", got)
	}
	if got := NewEnv().Title; got != "Example Board" {
		t.Errorf("NewEnv().Title = %q", got)
	}
}

func TestFillKeepsNoBreadcrumb(t *testing.T) {
	Use(testTable(), "Example Board")

	filled := For("Session").Fill(map[string]string{})
	if filled.HasBreadcrumb() {
		t.Error("Fill dropped NoBreadcrumb")
	}
	if got := filled.AsURL(); got != "session" {
		t.Errorf("AsURL() of a filled route = %q", got)
	}
}

func TestSetHierarchyDropsThePreparedValues(t *testing.T) {
	Use(testTable(), "Example Board")

	r := For("AdminUsers")
	r.SetHierarchy([]string{"root", "admin"})
	if got := r.AsURL(); got != "admin" {
		t.Errorf("AsURL() after SetHierarchy = %q", got)
	}
	if got := r.Parent(); got != "Root" {
		t.Errorf("Parent() after SetHierarchy = %q", got)
	}
}

func TestCollidesWithRoute(t *testing.T) {
	Use(testTable(), "Example Board")

	for path, want := range map[string]bool{
		"/admin/x":      true,
		"/Session":      true,
		"/_hosting/aws": true,
		"/":             true,
		"/media/a.png":  false,
	} {
		if got := CollidesWithRoute(path); got != want {
			t.Errorf("CollidesWithRoute(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestRouteControllerSatisfiesIRouteController(t *testing.T) {
	var c IRouteController = &RouteController{Path: "/x", Env: NewEnv()}

	if c.GetPath() != "/x" || c.GetEnv() == nil || c.GetRuntime() != nil {
		t.Errorf("GetPath() = %q, GetEnv() = %v, GetRuntime() = %v", c.GetPath(), c.GetEnv(), c.GetRuntime())
	}
}

func BenchmarkParentRoute(b *testing.B) {
	Use(testTable(), "Example Board")
	b.ReportAllocs()

	for b.Loop() {
		For("CategoriesForumsTopics").ParentRoute()
	}
}
