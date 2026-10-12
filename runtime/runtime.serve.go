package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"xn--gckvb8fzb.com/glides/errs"
)

type IProcess interface {
	Startup() error
	Run() error
	Shutdown() error
}

func (rt *Runtime) Serve(ctx context.Context, processes ...IProcess) (err error) {
	for _, process := range processes {
		if process == nil {
			return errs.ErrIfaceInvalid
		}
	}

	var started []IProcess
	for _, process := range processes {
		rt.Debug("startup", processName(process))
		if err = process.Startup(); err != nil {
			rt.Error("status", "error", "process", processName(process), "error", err)
			return errors.Join(err, rt.shutdownProcesses(started))
		}
		started = append(started, process)
	}

	results := make(chan error, len(started))
	for _, process := range started {
		rt.Debug("run", processName(process))
		go func() { results <- process.Run() }()
	}

	select {
	case <-ctx.Done():
		rt.Info("status", "stopping", "reason", ctx.Err())
	case err = <-results:
		rt.Info("status", "stopping", "reason", "process returned", "error", err)
	}

	return errors.Join(err, rt.shutdownProcesses(started))
}

func (rt *Runtime) shutdownProcesses(processes []IProcess) error {
	var failures []error

	for _, process := range slices.Backward(processes) {
		name := processName(process)
		rt.Debug("shutdown", name)
		if err := process.Shutdown(); err != nil {
			rt.Error("status", "error", "process", name, "error", err)
			failures = append(failures, err)
		}
	}

	return errors.Join(failures...)
}

func processName(process IProcess) string {
	return fmt.Sprintf("%T", process)
}
