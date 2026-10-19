package handler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/config"
)

func testRuntime(t *testing.T) *runtime.Runtime {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte("[Logging]\nLevel = \"error\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rt, err := runtime.New(runtime.Opts{Cfgstr: path})
	if err != nil {
		t.Fatal(err)
	}

	return rt
}

func TestTargetRunsTheHandlersItWasGiven(t *testing.T) {
	target, err := New(testRuntime(t), config.Target{ID: "jobs", Type: config.TargetTypeHandler})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err = target.Load(); err != nil {
		t.Errorf("Load = %v", err)
	}
	if err = target.Run(); err != nil {
		t.Errorf("Run = %v", err)
	}

	var payload string
	target.Handle(confirmation, signup, func(ctx context.Context, j asyncjob.AsyncJob) error {
		payload = string(j.Payload)
		return nil
	})

	if err = target.Execute(context.Background(), asyncjob.AsyncJob{
		Type: confirmation, SubType: signup, Payload: []byte(`"asset"`),
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if payload != `"asset"` {
		t.Errorf("the handler saw the payload %q, want %q", payload, `"asset"`)
	}

	err = target.Execute(context.Background(), asyncjob.AsyncJob{Type: notification, SubType: reply})
	if !errors.Is(err, errs.ErrJobTypeInvalid) {
		t.Errorf("Execute of a type without handlers = %v, want %v", err, errs.ErrJobTypeInvalid)
	}

	if err = target.Shutdown(); err != nil {
		t.Errorf("Shutdown = %v", err)
	}
}
