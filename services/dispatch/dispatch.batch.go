package dispatch

import (
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
	for _, payload := range payloads {
		targetID := r.TargetIDFor(isJID(payload))
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

	return nil
}
