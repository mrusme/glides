package dispatch

import (
	"fmt"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
)

func Batch[P any](
	disp *Dispatch,
	jobType asyncjob.JobType,
	jobSubType asyncjob.JobSubType,
	payloads []P,
	isJID func(payload P) bool,
) (err error) {
	r, err := disp.routing()
	if err != nil {
		return err
	}

	byTarget := make(map[string][]P)
	unrouted := 0
	for _, payload := range payloads {
		targetID := r.TargetIDFor(isJID(payload))
		if targetID == "" {
			unrouted++
			continue
		}
		byTarget[targetID] = append(byTarget[targetID], payload)
	}

	for targetID, targetPayloads := range byTarget {
		j := asyncjob.New(targetID, jobType, jobSubType)
		if err = j.SetPayload(targetPayloads); err != nil {
			return err
		}

		if err = disp.Job(j); err != nil {
			return err
		}
	}

	if unrouted > 0 {
		return fmt.Errorf("%w: %d of %d payloads have no target",
			errs.ErrTargetIDNotFound, unrouted, len(payloads))
	}

	return nil
}
