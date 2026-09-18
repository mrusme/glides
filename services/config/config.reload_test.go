package config

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func eventually(t *testing.T, what string, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("%s did not happen within 5s", what)
}

func TestReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.toml")
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("[General]\nMode = \"production\"\n")
	cfg, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	var failures atomic.Int32
	cfg.OnReloadError(func(error) { failures.Add(1) })

	if err = cfg.Startup(); err != nil {
		t.Fatal(err)
	}
	defer cfg.Shutdown()

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if mode := cfg.GeneralMode(); mode != "production" && mode != "development" {
					t.Errorf("a reader saw the mode %q", mode)
					return
				}
			}
		}()
	}

	write("[General]\nMode = \"development\"\n")
	eventually(t, "the reload", func() bool { return cfg.GeneralMode() == "development" })

	write("[General\nMode = broken")
	eventually(t, "the report of a failed reload", func() bool { return failures.Load() > 0 })
	if mode := cfg.GeneralMode(); mode != "development" {
		t.Errorf("a file that doesn't parse replaced the configuration, the mode is %q", mode)
	}

	close(stop)
	readers.Wait()
}
