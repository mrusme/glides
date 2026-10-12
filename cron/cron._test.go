package cron

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/runtime"
	servicescron "xn--gckvb8fzb.com/glides/services/cron"
	"xn--gckvb8fzb.com/glides/services/dispatch"
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

func TestShutdownWithoutRunReturns(t *testing.T) {
	crn, err := New(testRuntime(t))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- crn.Shutdown() }()

	select {
	case err = <-done:
		if err != nil {
			t.Errorf("Shutdown = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown without a Run blocked")
	}

	if err = crn.Run(); err != nil {
		t.Errorf("Run after Shutdown = %v, want nil and an immediate return", err)
	}
}

func TestStartupNeedsTheCronAndDispatchServices(t *testing.T) {
	rt := testRuntime(t)
	crn, err := New(rt)
	if err != nil {
		t.Fatal(err)
	}

	if err = crn.Startup(); !errors.Is(err, errs.ErrServiceNotFound) {
		t.Errorf("Startup without services = %v", err)
	}

	if _, err = servicescron.Register(rt); err != nil {
		t.Fatal(err)
	}
	if err = crn.Startup(); !errors.Is(err, errs.ErrServiceNotFound) {
		t.Errorf("Startup without dispatch = %v", err)
	}

	if _, err = dispatch.Register(rt); err != nil {
		t.Fatal(err)
	}
	if err = crn.Startup(); err != nil {
		t.Errorf("Startup with both services = %v", err)
	}
}
