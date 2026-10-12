package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/models/asyncjob/cron/cronrun"
	"xn--gckvb8fzb.com/glides/services/config"
)

const (
	envRedis     = "GLIDES_TEST_REDIS"
	defaultQueue = "default"
)

func closedPort(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	return addr
}

func live(t *testing.T) (config.Redis, *asynq.Inspector) {
	t.Helper()

	addr := os.Getenv(envRedis)
	if addr == "" {
		t.Skipf("%s is not set", envRedis)
	}

	cfg := config.Redis{Addrs: []string{addr}}
	inspector := asynq.NewInspector(RedisConnOpt(cfg))
	t.Cleanup(func() { inspector.Close() })

	return cfg, inspector
}

func started(t *testing.T, cfg config.Redis) *Dispatch {
	t.Helper()

	disp, err := New(cfg, config.Targets{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err = disp.Startup(); err != nil {
		t.Fatalf("Startup: %v", err)
	}
	t.Cleanup(func() { disp.Shutdown() })

	return disp
}

func TestStartupFailsWithoutARedis(t *testing.T) {
	disp, err := New(config.Redis{Addrs: []string{closedPort(t)}}, config.Targets{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err = disp.Startup(); err == nil {
		t.Fatal("Startup succeeded with nothing listening")
	}
	if err = disp.Job(asyncjob.New("t", "type", "sub")); !errors.Is(err, errs.ErrDispatchNotStarted) {
		t.Errorf("Job after a failed Startup = %v, want %v", err, errs.ErrDispatchNotStarted)
	}
	if err = disp.Shutdown(); err != nil {
		t.Errorf("Shutdown after a failed Startup = %v", err)
	}
}

func TestStartupPingsRedisAndJobEnqueues(t *testing.T) {
	cfg, inspector := live(t)
	disp := started(t, cfg)

	id := fmt.Sprintf("glides-test-%d", time.Now().UnixNano())
	j := asyncjob.New("glides-test-target", "glides-test", "enqueue")
	if err := disp.Job(j, asynq.TaskID(id)); err != nil {
		t.Fatalf("Job: %v", err)
	}
	t.Cleanup(func() { inspector.DeleteTask(defaultQueue, id) })

	info, err := inspector.GetTaskInfo(defaultQueue, id)
	if err != nil {
		t.Fatalf("GetTaskInfo: %v", err)
	}
	if info.Type != TaskJob || info.State != asynq.TaskStatePending || info.MaxRetry != 5 {
		t.Errorf("task = %s %v with %d retries, want %s pending with 5",
			info.Type, info.State, info.MaxRetry, TaskJob)
	}

	var got asyncjob.AsyncJob
	if err = json.Unmarshal(info.Payload, &got); err != nil || got.TargetID != j.TargetID {
		t.Errorf("payload = %+v, %v, want the job for %q", got, err, j.TargetID)
	}

	if err = inspector.DeleteTask(defaultQueue, id); err != nil {
		t.Errorf("DeleteTask: %v", err)
	}
	if err = disp.Shutdown(); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	if err = disp.Job(j); !errors.Is(err, errs.ErrDispatchNotStarted) {
		t.Errorf("Job after Shutdown = %v, want %v", err, errs.ErrDispatchNotStarted)
	}
}

func TestCronRunIsEnqueuedOncePerSchedule(t *testing.T) {
	cfg, inspector := live(t)
	disp := started(t, cfg)

	fn := fmt.Sprintf("glides-test-%d", time.Now().UnixNano())
	at := time.Now().Truncate(time.Second)
	run := cronrun.New(fn, at)
	t.Cleanup(func() { inspector.DeleteTask(QueueCron, run.TaskID()) })

	if err := disp.CronRun(run); err != nil {
		t.Fatalf("CronRun: %v", err)
	}
	if err := disp.CronRun(cronrun.New(fn, at)); err != nil {
		t.Errorf("second CronRun for the same schedule = %v, want nil", err)
	}

	info, err := inspector.GetTaskInfo(QueueCron, run.TaskID())
	if err != nil {
		t.Fatalf("GetTaskInfo: %v", err)
	}
	if info.Type != TaskCron || info.Queue != QueueCron || info.Retention != CronRetention {
		t.Errorf("task = %s in %q kept %v, want %s in %q kept %v",
			info.Type, info.Queue, info.Retention, TaskCron, QueueCron, CronRetention)
	}

	pending, err := inspector.ListPendingTasks(QueueCron)
	if err != nil {
		t.Fatalf("ListPendingTasks: %v", err)
	}
	runs := 0
	for _, task := range pending {
		if task.ID == run.TaskID() {
			runs++
		}
	}
	if runs != 1 {
		t.Errorf("%d tasks for the schedule, want 1", runs)
	}
}
