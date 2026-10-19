package targets

import (
	"context"
	"embed"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/worker/targets/handler"
)

func testRuntime(t *testing.T, targets string) *runtime.Runtime {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte("[Logging]\nLevel = \"error\"\n"+targets), 0o600); err != nil {
		t.Fatal(err)
	}

	rt, err := runtime.New(runtime.Opts{Cfgstr: path})
	if err != nil {
		t.Fatal(err)
	}

	return rt
}

const twoDebugTargets = `
[[Target]]
ID = "debug-email"
Type = "debug"
Debug.Emulates = "email"

[[Target]]
ID = "debug-xmpp"
Type = "debug"
Debug.Emulates = "xmpp"
`

func TestNewRejectsDuplicateIDs(t *testing.T) {
	rt := testRuntime(t, `
[[Target]]
ID = "debug"
Type = "debug"
Debug.Emulates = "email"

[[Target]]
ID = "debug"
Type = "debug"
Debug.Emulates = "xmpp"
`)
	rt.AddEmbed("templates", &embed.FS{})

	if _, err := New(rt, nil); !errors.Is(err, errs.ErrTargetIDDuplicate) {
		t.Errorf("New with two targets of one id = %v", err)
	}
}

func TestNewRejectsAnUnknownType(t *testing.T) {
	rt := testRuntime(t, "[[Target]]\nID = \"pigeons\"\nType = \"pigeon\"\n")
	rt.AddEmbed("templates", &embed.FS{})

	if _, err := New(rt, nil); !errors.Is(err, errs.ErrNoSuchTargetType) {
		t.Errorf("New with an unknown type = %v", err)
	}
}

func TestNewNeedsTheTemplatesEmbed(t *testing.T) {
	rt := testRuntime(t, twoDebugTargets)

	if _, err := New(rt, nil); !errors.Is(err, errs.ErrEmbedNotFound) {
		t.Errorf("New without the templates embed = %v", err)
	}
}

func TestRegisterSeesEveryTargetAndExecuteRoutesByID(t *testing.T) {
	rt := testRuntime(t, twoDebugTargets)
	rt.AddEmbed("templates", &embed.FS{})

	var seen []string
	ts, err := New(rt, func(id string, target ITarget) error {
		seen = append(seen, id)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"debug-email", "debug-xmpp"}; !slices.Equal(seen, want) {
		t.Errorf("Register saw %v, want %v", seen, want)
	}

	err = ts.Execute(context.Background(), "missing", asyncjob.AsyncJob{})
	if !errors.Is(err, errs.ErrTargetIDNotFound) {
		t.Errorf("Execute on an unknown target = %v", err)
	}

	err = ts.Execute(context.Background(), "debug-email", asyncjob.AsyncJob{Type: "nothing"})
	if !errors.Is(err, errs.ErrJobTypeInvalid) {
		t.Errorf("Execute of an unregistered job type = %v", err)
	}

	if err = ts.LoadAll(); err != nil {
		t.Errorf("LoadAll = %v", err)
	}
	if ok, failures := ts.ShutdownAll(); !ok {
		t.Errorf("ShutdownAll = %v", failures)
	}
}

func TestNewBuildsAHandlerTargetWithoutTheTemplatesEmbed(t *testing.T) {
	rt := testRuntime(t, "[[Target]]\nID = \"jobs\"\nType = \"handler\"\n")

	const thumbnails asyncjob.JobType = "thumbnails"
	const run asyncjob.JobSubType = "run"

	var ran bool
	ts, err := New(rt, func(id string, target ITarget) error {
		h, ok := target.(*handler.Target)
		if !ok {
			t.Errorf("target %q is a %T, want a *handler.Target", id, target)
			return nil
		}
		h.Handle(thumbnails, run, func(ctx context.Context, j asyncjob.AsyncJob) error {
			ran = true
			return nil
		})
		return nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err = ts.LoadAll(); err != nil {
		t.Errorf("LoadAll = %v", err)
	}
	if err = ts.RunAll(); err != nil {
		t.Errorf("RunAll = %v", err)
	}

	if err = ts.Execute(context.Background(), "jobs", asyncjob.AsyncJob{Type: thumbnails, SubType: run}); err != nil {
		t.Errorf("Execute = %v", err)
	}
	if !ran {
		t.Error("the handler registered through Register did not run")
	}

	if ok, failures := ts.ShutdownAll(); !ok {
		t.Errorf("ShutdownAll = %v", failures)
	}
}
