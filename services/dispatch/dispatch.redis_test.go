package dispatch

import (
	"context"
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

func TestJobUniqueKeepsAQueuedTaskAndReplacesAFinishedOne(t *testing.T) {
	cfg, inspector := live(t)
	disp := started(t, cfg)

	queue := fmt.Sprintf("glides-test-%d", time.Now().UnixNano())
	t.Cleanup(func() { inspector.DeleteQueue(queue, true) })
	const id = "unique"

	held := func() (asynq.TaskState, string) {
		t.Helper()

		info, err := inspector.GetTaskInfo(queue, id)
		if err != nil {
			t.Fatalf("GetTaskInfo: %v", err)
		}
		var got asyncjob.AsyncJob
		if err = json.Unmarshal(info.Payload, &got); err != nil {
			t.Fatalf("payload: %v", err)
		}

		return info.State, got.ID.String()
	}
	newJob := func() *asyncjob.AsyncJob {
		return asyncjob.New("glides-test-target", "glides-test", "unique")
	}

	first := newJob()
	if err := disp.JobUnique(first, id, asynq.Queue(queue)); err != nil {
		t.Fatalf("JobUnique: %v", err)
	}
	if err := disp.JobUnique(newJob(), id, asynq.Queue(queue)); err != nil {
		t.Fatalf("JobUnique while the task is pending = %v, want nil", err)
	}
	if state, jobID := held(); state != asynq.TaskStatePending || jobID != first.ID.String() {
		t.Errorf("the task is %v holding job %s, want the first job %s still pending", state, jobID, first.ID)
	}

	if err := inspector.ArchiveTask(queue, id); err != nil {
		t.Fatalf("ArchiveTask: %v", err)
	}
	if err := disp.Job(newJob(), asynq.Queue(queue), asynq.TaskID(id)); !errors.Is(err, asynq.ErrTaskIDConflict) {
		t.Errorf("Job with the id of an archived task = %v, want %v", err, asynq.ErrTaskIDConflict)
	}
	retried := newJob()
	if err := disp.JobUnique(retried, id, asynq.Queue(queue)); err != nil {
		t.Fatalf("JobUnique while the task is archived: %v", err)
	}
	if state, jobID := held(); state != asynq.TaskStatePending || jobID != retried.ID.String() {
		t.Errorf("the task is %v holding job %s, want the new job %s pending", state, jobID, retried.ID)
	}

	if err := inspector.DeleteTask(queue, id); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if err := disp.JobUnique(newJob(), id, asynq.Queue(queue), asynq.Retention(time.Hour)); err != nil {
		t.Fatalf("JobUnique with a retention: %v", err)
	}

	srv := asynq.NewServer(RedisConnOpt(cfg), asynq.Config{
		Queues:      map[string]int{queue: 1},
		Concurrency: 1,
		LogLevel:    asynq.FatalLevel,
	})
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskJob, func(ctx context.Context, task *asynq.Task) error { return nil })
	if err := srv.Start(mux); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if state, _ := held(); state == asynq.TaskStateCompleted {
			break
		}
		if time.Now().After(deadline) {
			srv.Shutdown()
			t.Fatal("the task did not complete within 10 seconds")
		}
		time.Sleep(50 * time.Millisecond)
	}
	srv.Shutdown()

	again := newJob()
	if err := disp.JobUnique(again, id, asynq.Queue(queue)); err != nil {
		t.Fatalf("JobUnique while a completed task is retained: %v", err)
	}
	if state, jobID := held(); state != asynq.TaskStatePending || jobID != again.ID.String() {
		t.Errorf("the task is %v holding job %s, want the new job %s pending", state, jobID, again.ID)
	}
}
