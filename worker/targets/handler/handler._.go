package handler

import (
	"fmt"
	"sync"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
)

type Handler func(j asyncjob.AsyncJob) error

type Message struct {
	Username string
	Address  string
	Lang     string
	Subject  string
}

type Key struct {
	Type    asyncjob.JobType
	SubType asyncjob.JobSubType
}

type Handlers struct {
	mu       sync.RWMutex
	handlers map[Key]Handler
	types    map[asyncjob.JobType]bool
}

func (h *Handlers) Handle(
	jobType asyncjob.JobType,
	jobSubType asyncjob.JobSubType,
	fn Handler,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.handlers == nil {
		h.handlers = make(map[Key]Handler)
		h.types = make(map[asyncjob.JobType]bool)
	}

	h.handlers[Key{Type: jobType, SubType: jobSubType}] = fn
	h.types[jobType] = true
}

func (h *Handlers) Execute(j asyncjob.AsyncJob) error {
	h.mu.RLock()
	fn, ok := h.handlers[Key{Type: j.Type, SubType: j.SubType}]
	known := h.types[j.Type]
	h.mu.RUnlock()

	if !ok {
		if !known {
			return fmt.Errorf("%w: %s", errs.ErrJobTypeInvalid, j.Type)
		}
		return fmt.Errorf("%w: %s/%s",
			errs.ErrJobSubTypeInvalid, j.Type, j.SubType)
	}

	return fn(j)
}
