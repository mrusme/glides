package dispatch

import (
	"errors"

	"github.com/hibiken/asynq"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
)

const QueueDefault string = "default"

func (disp *Dispatch) JobUnique(
	j *asyncjob.AsyncJob,
	id string,
	opts ...asynq.Option,
) (err error) {
	opts = append(opts, asynq.TaskID(id))

	if err = disp.Job(j, opts...); !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}

	queue := queueOf(opts)

	info, err := disp.ai.GetTaskInfo(queue, id)
	switch {
	case errors.Is(err, asynq.ErrTaskNotFound), errors.Is(err, asynq.ErrQueueNotFound):
	case err != nil:
		return err
	case info.State != asynq.TaskStateArchived && info.State != asynq.TaskStateCompleted:
		return nil
	default:
		if err = disp.ai.DeleteTask(queue, id); err != nil && !errors.Is(err, asynq.ErrTaskNotFound) {
			return err
		}
	}

	if err = disp.Job(j, opts...); errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}

	return err
}

func queueOf(opts []asynq.Option) string {
	queue := QueueDefault

	for _, opt := range opts {
		if opt.Type() != asynq.QueueOpt {
			continue
		}
		if name, ok := opt.Value().(string); ok {
			queue = name
		}
	}

	return queue
}
