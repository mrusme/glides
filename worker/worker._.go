package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/hibiken/asynq"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/dispatch"
	"xn--gckvb8fzb.com/glides/worker/targets"
)

type Worker struct {
	rt       *runtime.Runtime
	ts       *targets.Targets
	register targets.Register

	mu          sync.Mutex
	as          *asynq.Server
	concurrency int
	running     bool
	stop        chan struct{}
	stopOnce    sync.Once
}

func New(
	rt *runtime.Runtime,
	register targets.Register,
) (*Worker, error) {
	wrk := new(Worker)

	wrk.rt = rt
	wrk.register = register
	wrk.stop = make(chan struct{})

	return wrk, nil
}

func (wrk *Worker) SetConcurrency(n int) {
	wrk.mu.Lock()
	wrk.concurrency = n
	wrk.mu.Unlock()
}

func (wrk *Worker) Startup() (err error) {
	if wrk.ts, err = targets.New(wrk.rt, wrk.register); err != nil {
		return err
	}

	return wrk.ts.LoadAll()
}

func (wrk *Worker) Run() (err error) {
	redisCfg, err := wrk.rt.Config().Redis()
	if err != nil {
		return err
	}

	wrk.mu.Lock()
	select {
	case <-wrk.stop:
		wrk.mu.Unlock()
		return nil
	default:
	}
	if wrk.ts == nil {
		wrk.mu.Unlock()
		return fmt.Errorf("%w: worker was not started", errs.ErrProcessState)
	}
	if wrk.running {
		wrk.mu.Unlock()
		return fmt.Errorf("%w: worker is running", errs.ErrProcessState)
	}

	if err = wrk.ts.RunAll(); err != nil {
		wrk.mu.Unlock()
		return err
	}

	as := asynq.NewServer(
		dispatch.RedisConnOpt(redisCfg),
		asynq.Config{
			Logger:      wrk.rt.ALogger,
			Concurrency: wrk.concurrency,
		},
	)
	mux := asynq.NewServeMux()
	mux.HandleFunc(dispatch.TaskJob, wrk.HandleJob)

	if err = as.Start(mux); err != nil {
		wrk.mu.Unlock()
		return err
	}
	wrk.as = as
	wrk.running = true
	wrk.mu.Unlock()

	<-wrk.stop

	return nil
}

func (wrk *Worker) Shutdown() error {
	wrk.mu.Lock()
	wrk.stopOnce.Do(func() { close(wrk.stop) })
	as := wrk.as
	wrk.as = nil
	wrk.running = false
	wrk.mu.Unlock()

	if as != nil {
		as.Shutdown()
	}

	if wrk.ts == nil {
		return nil
	}

	ok, failures := wrk.ts.ShutdownAll()
	if ok {
		return nil
	}

	joined := make([]error, 0, len(failures))
	for id, ferr := range failures {
		wrk.rt.Error("status", "error", "target", id, "error", ferr)
		joined = append(joined, ferr)
	}

	return errors.Join(joined...)
}

func (wrk *Worker) HandleJob(ctx context.Context, t *asynq.Task) error {
	var job asyncjob.AsyncJob
	if err := json.Unmarshal(t.Payload(), &job); err != nil {
		return err
	}

	wrk.rt.Debug("status", "working",
		"id", job.ID, "target", job.TargetID,
		"type", job.Type, "sub_type", job.SubType)

	if err := wrk.ts.Execute(ctx, job.TargetID, job); err != nil {
		wrk.rt.Error("error", err, "id", job.ID, "target", job.TargetID)
		return err
	}

	return nil
}
