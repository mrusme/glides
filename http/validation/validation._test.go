package validation

import "testing"

func TestSlug(t *testing.T) {
	type form struct {
		Slug string `validate:"required,slug"`
	}

	v := NewStructValidator(New())
	for value, valid := range map[string]bool{
		"hello-world": true,
		"abc123":      true,
		"Hello":       false,
		"a_b":         false,
		"a b":         false,
		"":            false,
	} {
		err := v.Validate(form{Slug: value})
		if (err == nil) != valid {
			t.Errorf("Validate(%q) = %v, want valid=%v", value, err, valid)
		}
	}

	allocs := testing.AllocsPerRun(100, func() { v.Validate(form{Slug: "hello-world"}) })
	if allocs > 2 {
		t.Errorf("validating a slug costs %.0f allocations", allocs)
	}
}
