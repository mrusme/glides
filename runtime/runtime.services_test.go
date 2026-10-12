package runtime

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
)

type recorder struct {
	name     string
	events   *[]string
	fail     error
	failStop error
}

func (r *recorder) Startup() error {
	*r.events = append(*r.events, "start "+r.name)
	return r.fail
}

func (r *recorder) Shutdown() error {
	*r.events = append(*r.events, "stop "+r.name)
	return r.failStop
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func newTestRuntime(t *testing.T) *Runtime {
	t.Helper()

	rt, err := New(Opts{Cfgstr: writeConfig(t, "[Logging]\nLevel = \"error\"\n")})
	if err != nil {
		t.Fatal(err)
	}
	rt.logger = slog.New(slog.NewJSONHandler(new(bytes.Buffer), nil))

	return rt
}

func TestNewDefaultsTheLoggingLevel(t *testing.T) {
	rt, err := New(Opts{Cfgstr: writeConfig(t, "[General]\nMode = \"production\"\n")})
	if err != nil {
		t.Fatalf("New without a [Logging] section: %v", err)
	}
	if got := rt.GetLogLevel(); got != DefaultLoggingLevel {
		t.Errorf("GetLogLevel() = %v, want %v", got, DefaultLoggingLevel)
	}

	if _, err = New(Opts{Cfgstr: writeConfig(t, "[Logging]\nLevel = \"loud\"\n")}); err == nil {
		t.Error("New accepted an unknown logging level")
	}
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

func TestStartupFailureStopsWhatStarted(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	broken := errors.New("broken")
	rt.Register("a", &recorder{name: "a", events: &events})
	rt.Register("b", &recorder{name: "b", events: &events, fail: broken})
	rt.Register("c", &recorder{name: "c", events: &events})
	rt.OnShutdown(func() error { events = append(events, "stop hook"); return nil })

	if err := rt.Startup(); !errors.Is(err, broken) {
		t.Fatalf("Startup = %v, want the error of b", err)
	}

	if want := []string{"start a", "start b", "stop a"}; !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}

	if err := rt.Shutdown(); err != nil {
		t.Errorf("Shutdown after a failed Startup: %v", err)
	}
	if want := []string{"start a", "start b", "stop a"}; !slices.Equal(events, want) {
		t.Errorf("a second Shutdown stopped something again: %v", events)
	}
}

func TestStartupHookFailureStopsEverything(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	broken := errors.New("broken")
	rt.Register("a", &recorder{name: "a", events: &events})
	rt.OnStartup(func() error { events = append(events, "start hook"); return broken })
	rt.OnShutdown(func() error { events = append(events, "stop hook"); return nil })

	if err := rt.Startup(); !errors.Is(err, broken) {
		t.Fatalf("Startup = %v, want the error of the hook", err)
	}

	if want := []string{"start a", "start hook", "stop hook", "stop a"}; !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}

func TestShutdownIsIdempotentAndContinuesAfterAFailure(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	broken := errors.New("broken")
	rt.Register("a", &recorder{name: "a", events: &events})
	rt.Register("b", &recorder{name: "b", events: &events, failStop: broken})
	rt.Register("c", &recorder{name: "c", events: &events})

	if err := rt.Startup(); err != nil {
		t.Fatal(err)
	}
	if err := rt.Shutdown(); !errors.Is(err, broken) {
		t.Errorf("Shutdown = %v, want the error of b", err)
	}
	if want := []string{"start a", "start b", "start c", "stop c", "stop b", "stop a"}; !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}

	if err := rt.Shutdown(); err != nil {
		t.Errorf("a second Shutdown = %v, want nil", err)
	}
	if len(events) != 6 {
		t.Errorf("a second Shutdown stopped something again: %v", events)
	}
}

func TestShutdownBeforeStartupDoesNothing(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	rt.Register("a", &recorder{name: "a", events: &events})
	rt.OnShutdown(func() error { events = append(events, "stop hook"); return nil })

	if err := rt.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("Shutdown before Startup ran %v", events)
	}
}

func TestGetServiceReportsAMissingID(t *testing.T) {
	rt := newTestRuntime(t)

	if _, err := rt.GetService("nothing"); !errors.Is(err, errs.ErrServiceNotFound) {
		t.Errorf("GetService = %v", err)
	}
}
