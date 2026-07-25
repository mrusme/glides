package dispatch

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/services/config"
)

const (
	TaskJob   string = "job"
	TaskCron  string = "cron"
	QueueCron string = "cron"
)

type Routing struct {
	EmailTargetID string
	XMPPTargetID  string
}

func (r Routing) TargetIDFor(isJID bool) string {
	if isJID {
		return r.XMPPTargetID
	}

	return r.EmailTargetID
}

type Resolver interface {
	Routing() (Routing, error)
}

type Dispatch struct {
	cfg      config.Redis
	targets  config.Targets
	resolver Resolver
	ac       *asynq.Client
}

func New(
	cfg config.Redis,
	targets config.Targets,
) (disp *Dispatch, err error) {
	disp = new(Dispatch)

	disp.cfg = cfg
	disp.targets = targets
	disp.ac = nil

	return disp, nil
}

func (disp *Dispatch) SetResolver(resolver Resolver) {
	disp.resolver = resolver
}

func (disp *Dispatch) DebugTargetIDFor(channel string) string {
	for _, target := range disp.targets {
		if target.IsDebug() && target.Serves(channel) {
			return target.ID
		}
	}

	return ""
}

func (disp *Dispatch) routing() (r Routing, err error) {
	if disp.resolver == nil {
		return r, errs.ErrDispatchResolverMissing
	}

	if r, err = disp.resolver.Routing(); err != nil {
		return r, err
	}

	if r.EmailTargetID == "" {
		r.EmailTargetID = disp.DebugTargetIDFor(config.TargetTypeEmail)
	}

	if r.XMPPTargetID == "" {
		r.XMPPTargetID = disp.DebugTargetIDFor(config.TargetTypeXMPP)
	}

	return r, nil
}

func (disp *Dispatch) Startup() (err error) {
	disp.ac = asynq.NewClient(RedisConnOpt(disp.cfg))

	return nil
}

func (disp *Dispatch) Shutdown() (err error) {
	if disp.ac != nil {
		disp.ac.Close()
	}
	return nil
}

func (disp *Dispatch) Job(
	j *asyncjob.AsyncJob,
	opts ...asynq.Option,
) (err error) {
	return disp.enqueue(j, TaskJob, opts...)
}

func (disp *Dispatch) enqueue(
	j *asyncjob.AsyncJob,
	taskType string,
	opts ...asynq.Option,
) (err error) {
	if _, err = j.SetID(); err != nil {
		return err
	}

	jj, err := json.Marshal(j)
	if err != nil {
		return err
	}

	task := asynq.NewTask(taskType, jj, append([]asynq.Option{
		asynq.MaxRetry(5),
		asynq.Timeout(30 * time.Minute),
	}, opts...)...)

	_, err = disp.ac.Enqueue(task)

	return err
}
