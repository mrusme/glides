package runtime

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type recorder struct {
	name   string
	events *[]string
	fail   error
}

func (r *recorder) Startup() error {
	*r.events = append(*r.events, "start "+r.name)
	return r.fail
}

func (r *recorder) Shutdown() error {
	*r.events = append(*r.events, "stop "+r.name)
	return nil
}

func newTestRuntime(t *testing.T) *Runtime {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte("[Logging]\nLevel = \"error\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rt, err := New(Opts{Cfgstr: path})
	if err != nil {
		t.Fatal(err)
	}
	rt.AddService("_logger", slog.New(slog.NewJSONHandler(new(bytes.Buffer), nil)))

	return rt
}

func TestServicesStartInOrderAndStopInReverse(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	for _, name := range []string{"a", "b", "c"} {
		rt.Register(name, &recorder{name: name, events: &events})
	}
	rt.OnStartup(func() error { events = append(events, "start hook"); return nil })
	rt.OnShutdown(func() error { events = append(events, "stop hook"); return nil })

	if err := rt.Startup(); err != nil {
		t.Fatal(err)
	}
	if err := rt.Shutdown(); err != nil {
		t.Fatal(err)
	}

	want := []string{"start a", "start b", "start c", "start hook", "stop hook", "stop c", "stop b", "stop a"}
	if !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}

func TestRegisterTwiceKeepsOnePlace(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	rt.Register("a", &recorder{name: "old", events: &events})
	rt.Register("b", &recorder{name: "b", events: &events})
	rt.Register("a", &recorder{name: "new", events: &events})

	if err := rt.Startup(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	if want := []string{"start new", "start b"}; !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}

func TestStartupStopsAtTheFirstFailure(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	broken := errors.New("broken")
	rt.Register("a", &recorder{name: "a", events: &events})
	rt.Register("b", &recorder{name: "b", events: &events, fail: broken})
	rt.Register("c", &recorder{name: "c", events: &events})

	if err := rt.Startup(); !errors.Is(err, broken) {
		t.Fatalf("Startup = %v, want the error of b", err)
	}
	defer rt.Shutdown()

	if want := []string{"start a", "start b"}; !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}
