package handler

import (
	"errors"
	"sync"
	"testing"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
)

const (
	confirmation asyncjob.JobType = "confirmation"
	notification asyncjob.JobType = "notification"
)

const (
	signup      asyncjob.JobSubType = "signup"
	emailChange asyncjob.JobSubType = "email_change"
	reply       asyncjob.JobSubType = "reply"
	mention     asyncjob.JobSubType = "mention"
)

func TestExecuteRoutesBySubType(t *testing.T) {
	h := new(Handlers)

	var got asyncjob.JobSubType
	h.Handle(confirmation, signup, func(j asyncjob.AsyncJob) error {
		got = signup
		return nil
	})
	h.Handle(confirmation, emailChange, func(j asyncjob.AsyncJob) error {
		got = emailChange
		return nil
	})

	if err := h.Execute(asyncjob.AsyncJob{
		Type: confirmation, SubType: emailChange,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got != emailChange {
		t.Errorf("ran %q, want %q", got, emailChange)
	}
}

func TestExecuteUnregisteredType(t *testing.T) {
	h := new(Handlers)
	h.Handle(confirmation, signup, func(j asyncjob.AsyncJob) error {
		return nil
	})

	err := h.Execute(asyncjob.AsyncJob{
		Type: notification, SubType: reply,
	})
	if !errors.Is(err, errs.ErrJobTypeInvalid) {
		t.Errorf("err = %v, want %v", err, errs.ErrJobTypeInvalid)
	}
}

func TestExecuteUnregisteredSubType(t *testing.T) {
	h := new(Handlers)
	h.Handle(notification, reply, func(j asyncjob.AsyncJob) error {
		return nil
	})

	err := h.Execute(asyncjob.AsyncJob{
		Type: notification, SubType: mention,
	})
	if !errors.Is(err, errs.ErrJobSubTypeInvalid) {
		t.Errorf("err = %v, want %v", err, errs.ErrJobSubTypeInvalid)
	}
}

func TestExecuteReturnsHandlerError(t *testing.T) {
	h := new(Handlers)
	want := errors.New("delivery failed")
	h.Handle(confirmation, signup, func(j asyncjob.AsyncJob) error {
		return want
	})

	err := h.Execute(asyncjob.AsyncJob{
		Type: confirmation, SubType: signup,
	})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestConcurrentExecute(t *testing.T) {
	h := new(Handlers)
	h.Handle(confirmation, signup, func(j asyncjob.AsyncJob) error {
		return nil
	})

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.Execute(asyncjob.AsyncJob{
				Type: confirmation, SubType: signup,
			}); err != nil {
				t.Errorf("Execute: %v", err)
			}
		}()
	}
	wg.Wait()
}
