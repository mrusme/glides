package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/models/asyncjob/cron/cronrun"
	"xn--gckvb8fzb.com/glides/runtime"
	servicescron "xn--gckvb8fzb.com/glides/services/cron"
	"xn--gckvb8fzb.com/glides/services/dispatch"
)

const CheckInterval time.Duration = 5 * time.Second

type Cron struct {
	rt   *runtime.Runtime
	next map[string]time.Time

	mu       sync.Mutex
	as       *asynq.Server
	running  bool
	stop     chan struct{}
	stopOnce sync.Once
	stopped  chan struct{}
}

func New(rt *runtime.Runtime) (crn *Cron, err error) {
	crn = new(Cron)

	crn.rt = rt
	crn.next = make(map[string]time.Time)
	crn.stop = make(chan struct{})
	crn.stopped = make(chan struct{})

	return crn, nil
}

func (crn *Cron) Startup() (err error) {
	if servicescron.From(crn.rt) == nil {
		return fmt.Errorf("%w: %s", errs.ErrServiceNotFound, servicescron.ServiceID)
	}
	if dispatch.From(crn.rt) == nil {
		return fmt.Errorf("%w: %s", errs.ErrServiceNotFound, dispatch.ServiceID)
	}

	return nil
}

func (crn *Cron) Run() (err error) {
	redisCfg, err := crn.rt.Config().Redis()
	if err != nil {
		return err
	}

	crn.mu.Lock()
	select {
	case <-crn.stop:
		crn.mu.Unlock()
		return nil
	default:
	}
	if crn.running {
		crn.mu.Unlock()
		return fmt.Errorf("%w: cron is running", errs.ErrProcessState)
	}

	as := asynq.NewServer(
		dispatch.RedisConnOpt(redisCfg),
		asynq.Config{
			Logger: crn.rt.ALogger,
			Queues: map[string]int{dispatch.QueueCron: 1},
		},
	)
	mux := asynq.NewServeMux()
	mux.HandleFunc(dispatch.TaskCron, crn.HandleJob)

	if err = as.Start(mux); err != nil {
		crn.mu.Unlock()
		crn.rt.Error("status", "error", "error", err)
		return err
	}
	crn.as = as
	crn.running = true
	crn.mu.Unlock()

	crn.rt.Info("status", "ok", "entries", len(servicescron.From(crn.rt).Entries()))

	crn.schedule()

	return nil
}

func (crn *Cron) Shutdown() (err error) {
	crn.mu.Lock()
	crn.stopOnce.Do(func() { close(crn.stop) })
	running := crn.running
	as := crn.as
	crn.running = false
	crn.as = nil
	crn.mu.Unlock()

	if running {
		<-crn.stopped
	}
	if as != nil {
		as.Shutdown()
	}

	return nil
}

func (crn *Cron) schedule() {
	defer close(crn.stopped)

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			crn.tick(now)
		case <-crn.stop:
			return
		}
	}
}

func (crn *Cron) tick(now time.Time) {
	for _, entry := range servicescron.From(crn.rt).Entries() {
		next, ok := crn.next[entry.ID]
		if !ok {
			crn.next[entry.ID] = entry.Schedule.Next(now)
			continue
		}

		if now.Before(next) {
			continue
		}

		crn.dispatch(entry.ID, next)
		crn.next[entry.ID] = entry.Schedule.Next(now)
	}
}

func (crn *Cron) dispatch(id string, scheduledAt time.Time) {
	crn.rt.Debug("dispatch", id, "scheduled_at", scheduledAt)

	if err := dispatch.From(crn.rt).CronRun(
		cronrun.New(id, scheduledAt),
	); err != nil {
		crn.rt.Error("error", err)
	}
}

func (crn *Cron) HandleJob(ctx context.Context, t *asynq.Task) (err error) {
	var job asyncjob.AsyncJob
	if err = json.Unmarshal(t.Payload(), &job); err != nil {
		return err
	}

	if job.Type != asyncjob.Cron || job.SubType != asyncjob.Run {
		return fmt.Errorf("%w: %s/%s",
			errs.ErrJobPayloadInvalid, job.Type, job.SubType)
	}

	var payload cronrun.CronRun
	if err = json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}

	entry, ok := servicescron.From(crn.rt).Get(payload.FunctionID)
	if !ok {
		return fmt.Errorf("%w: %s",
			errs.ErrCronFunctionIDNotFound, payload.FunctionID)
	}

	crn.rt.Info("status", "running", "function_id", entry.ID)

	if err = entry.Fn(ctx); err != nil {
		crn.rt.Error("error", err, "function_id", entry.ID)
		return err
	}

	crn.rt.Info("status", "ok", "function_id", entry.ID)

	return nil
}
