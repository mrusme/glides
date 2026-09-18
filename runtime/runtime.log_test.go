package runtime

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func testRuntime(buf *bytes.Buffer, level slog.Level) *Runtime {
	rt := new(Runtime)
	rt.services = make(map[string]any)
	rt.AddService("_logger", slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})))

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
