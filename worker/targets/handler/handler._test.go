package handler

import (
	"context"
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
	h.Handle(confirmation, signup, func(ctx context.Context, j asyncjob.AsyncJob) error {
		got = signup
		return nil
	})
	h.Handle(confirmation, emailChange, func(ctx context.Context, j asyncjob.AsyncJob) error {
		got = emailChange
		return nil
	})

	if err := h.Execute(context.Background(), asyncjob.AsyncJob{
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
	h.Handle(confirmation, signup, func(ctx context.Context, j asyncjob.AsyncJob) error {
		return nil
	})

	err := h.Execute(context.Background(), asyncjob.AsyncJob{
		Type: notification, SubType: reply,
	})
	if !errors.Is(err, errs.ErrJobTypeInvalid) {
		t.Errorf("err = %v, want %v", err, errs.ErrJobTypeInvalid)
	}
}

func TestExecuteUnregisteredSubType(t *testing.T) {
	h := new(Handlers)
	h.Handle(notification, reply, func(ctx context.Context, j asyncjob.AsyncJob) error {
		return nil
	})

	err := h.Execute(context.Background(), asyncjob.AsyncJob{
		Type: notification, SubType: mention,
	})
	if !errors.Is(err, errs.ErrJobSubTypeInvalid) {
		t.Errorf("err = %v, want %v", err, errs.ErrJobSubTypeInvalid)
	}
}

func TestExecuteReturnsHandlerErrorAndPassesTheContext(t *testing.T) {
	h := new(Handlers)
	want := errors.New("delivery failed")
	type keyType struct{}
	var seen any
	h.Handle(confirmation, signup, func(ctx context.Context, j asyncjob.AsyncJob) error {
		seen = ctx.Value(keyType{})
		return want
	})

	ctx := context.WithValue(context.Background(), keyType{}, "marker")
	err := h.Execute(ctx, asyncjob.AsyncJob{
		Type: confirmation, SubType: signup,
	})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
	if seen != "marker" {
		t.Errorf("the handler saw the context value %v, want marker", seen)
	}
}

func TestConcurrentExecute(t *testing.T) {
	h := new(Handlers)
	h.Handle(confirmation, signup, func(ctx context.Context, j asyncjob.AsyncJob) error {
		return nil
	})

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if err := h.Execute(context.Background(), asyncjob.AsyncJob{
				Type: confirmation, SubType: signup,
			}); err != nil {
				t.Errorf("Execute: %v", err)
			}
		})
	}
	wg.Wait()
}
