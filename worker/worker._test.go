package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/dispatch"
	"xn--gckvb8fzb.com/glides/worker/targets"
	"xn--gckvb8fzb.com/glides/worker/targets/handler"
)

const (
	envRedis      = "GLIDES_TEST_REDIS"
	redisDatabase = 14
	handlerTarget = "[[Target]]\nID = \"jobs\"\nType = \"handler\"\n"
)

func testRuntime(t *testing.T, extra string) *runtime.Runtime {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte("[Logging]\nLevel = \"error\"\n"+extra), 0o600); err != nil {
		t.Fatal(err)
	}

	rt, err := runtime.New(runtime.Opts{Cfgstr: path})
	if err != nil {
		t.Fatal(err)
	}

	return rt
}

func TestRunBeforeStartupIsAProcessStateError(t *testing.T) {
	wrk, err := New(testRuntime(t, handlerTarget), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err = wrk.Run(); !errors.Is(err, errs.ErrProcessState) {
		t.Errorf("Run before Startup = %v, want %v", err, errs.ErrProcessState)
	}
}

func TestShutdownWithoutRunReturns(t *testing.T) {
	wrk, err := New(testRuntime(t, handlerTarget), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = wrk.Startup(); err != nil {
		t.Fatalf("Startup with a handler target and no templates embed = %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- wrk.Shutdown() }()

	select {
	case err = <-done:
		if err != nil {
			t.Errorf("Shutdown = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown without a Run blocked")
	}

	if err = wrk.Run(); err != nil {
		t.Errorf("Run after Shutdown = %v, want nil and an immediate return", err)
	}
}

func TestSetConcurrencyLimitsTheJobsInFlight(t *testing.T) {
	addr := os.Getenv(envRedis)
	if addr == "" {
		t.Skipf("%s is not set", envRedis)
	}

	for _, concurrency := range []int{1, 2} {
		t.Run(fmt.Sprintf("concurrency %d", concurrency), func(t *testing.T) {
			if got := mostJobsInFlight(t, addr, concurrency, 4); got != int64(concurrency) {
				t.Errorf("%d jobs ran at once, want %d", got, concurrency)
			}
		})
	}
}

func mostJobsInFlight(t *testing.T, addr string, concurrency int, jobs int) int64 {
	t.Helper()

	rt := testRuntime(t, fmt.Sprintf("[Redis]\nAddresses = [%q]\nDatabase = %d\n%s",
		addr, redisDatabase, handlerTarget))

	redisCfg, err := rt.Config().Redis()
	if err != nil {
		t.Fatal(err)
	}
	targetsCfg, err := rt.Config().Targets()
	if err != nil {
		t.Fatal(err)
	}

	inspector := asynq.NewInspector(dispatch.RedisConnOpt(redisCfg))
	t.Cleanup(func() { inspector.Close() })
	queues, err := inspector.Queues()
	if err != nil {
		t.Fatalf("listing the queues of database %d: %v", redisDatabase, err)
	}
	if slices.Contains(queues, "default") {
		for _, clear := range []func(string) (int, error){
			inspector.DeleteAllPendingTasks,
			inspector.DeleteAllScheduledTasks,
			inspector.DeleteAllRetryTasks,
			inspector.DeleteAllArchivedTasks,
		} {
			if _, err = clear("default"); err != nil {
				t.Fatalf("clearing the default queue of database %d: %v", redisDatabase, err)
			}
		}
	}

	jobType := asyncjob.JobType(fmt.Sprintf("glides-test-%d", time.Now().UnixNano()))
	const run asyncjob.JobSubType = "run"

	var inFlight, most atomic.Int64
	done := make(chan struct{}, jobs)

	wrk, err := New(rt, func(id string, target targets.ITarget) error {
		if h, ok := target.(*handler.Target); ok {
			h.Handle(jobType, run, func(ctx context.Context, j asyncjob.AsyncJob) error {
				now := inFlight.Add(1)
				for {
					seen := most.Load()
					if now <= seen || most.CompareAndSwap(seen, now) {
						break
					}
				}
				time.Sleep(200 * time.Millisecond)
				inFlight.Add(-1)
				done <- struct{}{}
				return nil
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wrk.SetConcurrency(concurrency)

	disp, err := dispatch.New(redisCfg, targetsCfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = disp.Startup(); err != nil {
		t.Fatalf("dispatch Startup: %v", err)
	}
	t.Cleanup(func() { disp.Shutdown() })

	for range jobs {
		if err = disp.Job(asyncjob.New("jobs", jobType, run)); err != nil {
			t.Fatalf("Job: %v", err)
		}
	}

	if err = wrk.Startup(); err != nil {
		t.Fatalf("worker Startup: %v", err)
	}
	ran := make(chan error, 1)
	go func() { ran <- wrk.Run() }()

	timeout := time.After(20 * time.Second)
	for range jobs {
		select {
		case <-done:
		case <-timeout:
			wrk.Shutdown()
			t.Fatalf("the jobs did not finish within 20 seconds")
		}
	}

	if err = wrk.Shutdown(); err != nil {
		t.Errorf("Shutdown = %v", err)
	}
	if err = <-ran; err != nil {
		t.Errorf("Run = %v", err)
	}

	return most.Load()
}
