package cron

import (
	"context"
	"errors"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
)

func TestRegisterValidatesAndIndexes(t *testing.T) {
	crn, err := New()
	if err != nil {
		t.Fatal(err)
	}
	fn := func(context.Context) error { return nil }

	if err = crn.Register("", "* * * * *", fn); !errors.Is(err, errs.ErrCronFunctionInvalid) {
		t.Errorf("an empty id = %v", err)
	}
	if err = crn.Register("a", "* * * * *", nil); !errors.Is(err, errs.ErrCronFunctionInvalid) {
		t.Errorf("a nil function = %v", err)
	}
	if err = crn.Register("a", "every so often", fn); !errors.Is(err, errs.ErrCronFunctionInvalid) {
		t.Errorf("a broken spec = %v", err)
	}
	if err = crn.Register("a", "*/5 * * * *", fn); err != nil {
		t.Errorf("a valid entry = %v", err)
	}
	if err = crn.Register("a", "*/5 * * * *", fn); !errors.Is(err, errs.ErrCronFunctionIDExists) {
		t.Errorf("a duplicate id = %v", err)
	}

	entry, ok := crn.Get("a")
	if !ok || entry.Spec != "*/5 * * * *" || entry.Schedule == nil {
		t.Errorf("Get = %+v, %v", entry, ok)
	}
	if _, ok = crn.Get("b"); ok {
		t.Error("Get found an entry that was never registered")
	}
	if got := len(crn.Entries()); got != 1 {
		t.Errorf("Entries() has %d entries, want 1", got)
	}
}
