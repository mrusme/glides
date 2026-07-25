package route

import "testing"

func TestRouteFill(t *testing.T) {
	Use(Table{
		"Root":                   {Hierarchy: []string{"root"}},
		"Categories":             {Hierarchy: []string{"root", "_:categories"}},
		"CategoriesForums":       {Hierarchy: []string{"root", "_:categories", ":forums"}},
		"CategoriesForumsTopics": {Hierarchy: []string{"root", "_:categories", ":forums", ":topics"}},
	}, "Example Board")

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
