package runtime

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

func testRuntime(buf *bytes.Buffer, level slog.Level) *Runtime {
	rt := new(Runtime)
	rt.services = make(map[string]any)
	rt.logger = slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level}))

	return rt
}

func lastMessage(t *testing.T, buf *bytes.Buffer) string {
	t.Helper()

	var line struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("%v: %q", err, buf.String())
	}

	return line.Msg
}

func TestLogNamesItsCaller(t *testing.T) {
	var buf bytes.Buffer
	rt := testRuntime(&buf, slog.LevelDebug)

	for name, log := range map[string]func(...any){
		"Debug": rt.Debug, "Info": rt.Info, "Warn": rt.Warn, "Error": rt.Error,
	} {
		buf.Reset()
		log("status", "ok")
		if got := lastMessage(t, &buf); got != "runtime.TestLogNamesItsCaller" {
			t.Errorf("%s logged under %q", name, got)
		}
	}
}

type named struct{ rt *Runtime }

func (n *named) Method() { n.rt.Info("status", "ok") }

func TestLogNamesAMethodByItsType(t *testing.T) {
	var buf bytes.Buffer
	rt := testRuntime(&buf, slog.LevelDebug)

	(&named{rt: rt}).Method()
	if got := lastMessage(t, &buf); got != "named.Method" {
		t.Errorf("a method logged under %q", got)
	}

	buf.Reset()
	func() { rt.Info("status", "ok") }()
	if got := lastMessage(t, &buf); got != "runtime.TestLogNamesAMethodByItsType" {
		t.Errorf("a closure logged under %q", got)
	}
}

func TestLogSkipsDisabledLevels(t *testing.T) {
	var buf bytes.Buffer
	rt := testRuntime(&buf, slog.LevelWarn)

	rt.Debug("status", "ok")
	rt.Info("status", "ok")
	if buf.Len() != 0 {
		t.Errorf("a disabled level was logged: %q", buf.String())
	}

	rt.Warn("status", "ok")
	if buf.Len() == 0 {
		t.Error("an enabled level was not logged")
	}

	allocs := testing.AllocsPerRun(100, func() { rt.Debug("status", "ok") })
	if allocs > 1 {
		t.Errorf("a disabled Debug costs %.0f allocations", allocs)
	}
}

func TestLogResolvesTheCallerOnce(t *testing.T) {
	var buf bytes.Buffer
	rt := testRuntime(&buf, slog.LevelDebug)

	rt.Info("status", "warm")
	allocs := testing.AllocsPerRun(100, func() { buf.Reset(); rt.Info("status", "ok") })
	if allocs > 1 {
		t.Errorf("an enabled Info costs %.0f allocations after the first call", allocs)
	}
}

func TestShortFnName(t *testing.T) {
	for in, want := range map[string]string{
		"xn--gckvb8fzb.com/glides/runtime.New":                                "runtime.New",
		"xn--gckvb8fzb.com/glides/worker.(*Worker).HandleJob":                 "Worker.HandleJob",
		"xn--gckvb8fzb.com/glides/worker/targets/debug.HandlerFor[...].func1": "debug.HandlerFor",
		"xn--gckvb8fzb.com/glides/runtime.(*Runtime).shutdown-range1":         "Runtime.shutdown",
		"main.main-range2.func1":                                              "main.main",
		"main.funcs.Run":                                                      "funcs.Run",
		"main.main":                                                           "main.main",
		"":                                                                    "Unknown",
		"nodots":                                                              "nodots",
	} {
		if got := shortFnName(in); got != want {
			t.Errorf("shortFnName(%q) = %q, want %q", in, got, want)
		}
	}
}

func BenchmarkInfo(b *testing.B) {
	rt := new(Runtime)
	rt.services = make(map[string]any)
	rt.logger = slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	b.ReportAllocs()

	for b.Loop() {
		rt.Info("status", "ok")
	}
}

func BenchmarkPlainSlogInfo(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	b.ReportAllocs()

	for b.Loop() {
		logger.Info("runtime.BenchmarkPlainSlogInfo", "status", "ok")
	}
}
