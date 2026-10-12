package route

import (
	"strings"
)

type Route struct {
	Hierarchy    []string
	NoBreadcrumb bool

	prepared bool
	url      string
	id       string
	title    string
	parent   string
}

type Table map[string]Route

var Routes Table = make(Table)

func Use(table Table, title string) {
	Routes = prepare(table)
	defaultTitle = title
}

func prepare(table Table) Table {
	ids := make(map[string]string, len(table))
	for id, r := range table {
		ids[key(r.Hierarchy)] = id
	}

	prepared := make(Table, len(table))
	for id, r := range table {
		r.url = joinURL(r.Hierarchy)
		r.id = joinID(r.Hierarchy)
		r.title = titleOf(r.id)
		if len(r.Hierarchy) >= 2 {
			r.parent = ids[key(r.Hierarchy[:len(r.Hierarchy)-1])]
		}
		r.prepared = true
		prepared[id] = r
	}

	return prepared
}

func key(hierarchy []string) string {
	return strings.Join(hierarchy, "\x00")
}

func joinURL(hierarchy []string) string {
	if len(hierarchy) <= 1 {
		return ""
	}

	return strings.Join(hierarchy[1:], "/")
}

func joinID(hierarchy []string) string {
	joined := strings.Join(hierarchy, "_")
	if strings.Contains(joined, ":") {
		return ""
	}

	return joined
}

func titleOf(id string) string {
	if id == "" {
		return ""
	}

	return id + "_title"
}

func (r Route) Len() int {
	return len(r.Hierarchy)
}

func (r Route) AsURL() string {
	if r.prepared {
		return r.url
	}

	return joinURL(r.Hierarchy)
}

func (r Route) AsID() string {
	if r.prepared {
		return r.id
	}

	return joinID(r.Hierarchy)
}

func (r Route) AsTitle() (title string) {
	if r.prepared {
		return r.title
	}

	return titleOf(joinID(r.Hierarchy))
}

func (r Route) Pathname() string {
	rl := r.Len()
	if rl == 0 {
		return ""
	}

	return r.Hierarchy[rl-1]
}

func (r Route) Parent() (id string) {
	if r.prepared {
		return r.parent
	}

	rl := r.Len()
	if rl < 2 {
		return ""
	}

	parentRoute := Route{Hierarchy: r.Hierarchy[:rl-1]}
	for key, rt := range Routes {
		if rt.Equals(parentRoute) {
			return key
		}
	}

	return ""
}

func (r Route) Equals(cmp Route) (eq bool) {
	if cmp.Len() != r.Len() {
		return false
	}
	for idx := range r.Hierarchy {
		if cmp.Hierarchy[idx] != r.Hierarchy[idx] {
			return false
		}
	}

	return true
}

func (r Route) ParentRoute() (rt Route) {
	id := r.Parent()
	if id == "" {
		return Route{}
	}

	return For(id)
}

func (r Route) HasBreadcrumb() bool {
	return r.NoBreadcrumb == false
}

func (r Route) Fill(params map[string]string) (rt Route) {
	filledHierarchy := make([]string, 0, len(r.Hierarchy))

	for _, segment := range r.Hierarchy {
		before, after, ok := strings.Cut(segment, ":")
		if ok {
			if filler, ok := params[after]; ok {
				filledHierarchy = append(filledHierarchy, before+filler)
				continue
			}
		}
		filledHierarchy = append(filledHierarchy, segment)
	}

	return Route{
		Hierarchy:    filledHierarchy,
		NoBreadcrumb: r.NoBreadcrumb,
	}
}

func (r *Route) SetHierarchy(h []string) {
	r.Hierarchy = h
	r.prepared = false
}

func For(id string) (r Route) {
	var ok bool

	if r, ok = Routes[id]; !ok {
		return Route{}
	}

	return r
}

func CollidesWithRoute(path string) bool {
	seg := strings.TrimPrefix(path, "/")
	if idx := strings.Index(seg, "/"); idx > -1 {
		seg = seg[:idx]
	}
	seg = strings.ToLower(seg)
	if seg == "" {
		return true
	}

	for _, r := range Routes {
		if r.Len() < 2 {
			continue
		}

		top := strings.ToLower(r.Hierarchy[1])
		before, _, ok := strings.Cut(top, ":")
		if !ok {
			if seg == top {
				return true
			}
			continue
		}

		prefix := before
		if prefix == "" || strings.HasPrefix(seg, prefix) {
			return true
		}
	}

	return false
}
