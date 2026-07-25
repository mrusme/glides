package asyncjob

import (
	"encoding/json"
	"reflect"

	"github.com/google/uuid"
	"xn--gckvb8fzb.com/glides/errs"
)

type JobType string

const (
	Cron JobType = "cron"
)

type JobSubType string

const (
	// Cron
	Run JobSubType = "run"
)

type AsyncJob struct {
	ID       uuid.UUID  `json:"id"`
	TargetID string     `json:"target_id"`
	Type     JobType    `json:"type"`
	SubType  JobSubType `json:"sub_type"`
	Batch    bool       `json:"batch"`
	Payload  []byte     `json:"payload"`
	Track    bool       `json:"track"`
}

func New(
	targetID string,
	jobType JobType,
	jobSubType JobSubType,
) (j *AsyncJob) {
	j = new(AsyncJob)
	j.TargetID = targetID
	j.Type = jobType
	j.SubType = jobSubType

	return j
}

func (j *AsyncJob) SetPayload(payload any) (err error) {
	if payload == nil {
		return errs.ErrJobPayloadInvalid
	}

	if j.Payload, err = json.Marshal(payload); err != nil {
		return err
	}

	j.Batch = reflect.ValueOf(payload).Kind() == reflect.Slice

	return nil
}

func (j *AsyncJob) SetID() (id uuid.UUID, err error) {
	if id, err = uuid.NewV7(); err != nil {
		return id, err
	}

	j.ID = id

	return j.ID, nil
}

func Payloads[P any](j AsyncJob) (payloads []P, err error) {
	if j.Batch {
		if err = json.Unmarshal(j.Payload, &payloads); err != nil {
			return nil, err
		}

		return payloads, nil
	}

	var payload P
	if err = json.Unmarshal(j.Payload, &payload); err != nil {
		return nil, err
	}

	return append(payloads, payload), nil
}
