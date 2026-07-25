package route

import (
	"strings"
)

type Route struct {
	Hierarchy    []string
	NoBreadcrumb bool
}

type Table map[string]Route

var Routes Table = make(Table)

func Use(table Table, title string) {
	Routes = table
	defaultTitle = title
}

func (r Route) Len() int {
	return len(r.Hierarchy)
}

func (r Route) AsURL() string {
	if r.Len() <= 1 {
		return ""
	}
	return strings.Join([]string(r.Hierarchy)[1:], "/")
}

func (r Route) AsID() string {
	joined := strings.Join([]string(r.Hierarchy), "_")
	if strings.Index(joined, ":") > -1 {
		return ""
	}
	return joined
}

func (r Route) AsTitle() (title string) {
	if title = r.AsID(); title == "" {
		return title
	}
	return title + "_title"
}

func (r Route) Pathname() string {
	rl := r.Len()
	if rl == 0 {
		return ""
	}

	return r.Hierarchy[rl-1]
}

func (r Route) Parent() (id string) {
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
	var filledHierarchy []string

	for _, segment := range r.Hierarchy {
		cidx := strings.Index(segment, ":")
		if cidx > -1 {
			segmentVar := segment[cidx+1:]
			if filler, ok := params[segmentVar]; ok {
				filledHierarchy = append(filledHierarchy, segment[0:cidx]+filler)
			} else {
				filledHierarchy = append(filledHierarchy, segment)
			}
		} else {
			filledHierarchy = append(filledHierarchy, segment)
		}
	}

	return Route{
		Hierarchy: filledHierarchy,
	}
}

func (r *Route) SetHierarchy(h []string) {
	r.Hierarchy = h
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
		cidx := strings.Index(top, ":")
		if cidx == -1 {
			if seg == top {
				return true
			}
			continue
		}

		prefix := top[:cidx]
		if prefix == "" || strings.HasPrefix(seg, prefix) {
			return true
		}
	}

	return false
}
