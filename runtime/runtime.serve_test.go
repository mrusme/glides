package runtime

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"xn--gckvb8fzb.com/glides/errs"
)

type process struct {
	name    string
	events  *[]string
	fail    error
	runErr  error
	stop    chan struct{}
	stopped chan struct{}
}

func newProcess(name string, events *[]string) *process {
	return &process{
		name:    name,
		events:  events,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (p *process) Startup() error {
	*p.events = append(*p.events, "start "+p.name)
	return p.fail
}

func (p *process) Run() error {
	if p.runErr != nil {
		return p.runErr
	}
	<-p.stop
	close(p.stopped)
	return nil
}

func (p *process) Shutdown() error {
	*p.events = append(*p.events, "stop "+p.name)
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	return nil
}

func TestServeStopsEveryProcessWhenTheContextEnds(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	a := newProcess("a", &events)
	b := newProcess("b", &events)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- rt.Serve(ctx, a, b) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the context ended")
	}

	if want := []string{"start a", "start b", "stop b", "stop a"}; !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
	<-a.stopped
	<-b.stopped
}

func TestServeStopsWhenAProcessFailsToStart(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	broken := errors.New("broken")
	a := newProcess("a", &events)
	b := newProcess("b", &events)
	b.fail = broken
	c := newProcess("c", &events)

	if err := rt.Serve(context.Background(), a, b, c); !errors.Is(err, broken) {
		t.Errorf("Serve = %v, want the error of b", err)
	}

	if want := []string{"start a", "start b", "stop a"}; !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}

func TestServeStopsWhenAProcessReturns(t *testing.T) {
	rt := newTestRuntime(t)

	var events []string
	broken := errors.New("listen failed")
	a := newProcess("a", &events)
	b := newProcess("b", &events)
	b.runErr = broken

	if err := rt.Serve(context.Background(), a, b); !errors.Is(err, broken) {
		t.Errorf("Serve = %v, want the error of b", err)
	}

	if want := []string{"start a", "start b", "stop b", "stop a"}; !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}

func TestServeRejectsANilProcess(t *testing.T) {
	rt := newTestRuntime(t)

	if err := rt.Serve(context.Background(), nil); !errors.Is(err, errs.ErrIfaceInvalid) {
		t.Errorf("Serve(nil) = %v", err)
	}
}
