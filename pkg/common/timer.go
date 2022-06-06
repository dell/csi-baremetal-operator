package common

import (
	"context"
	"time"
)

type handle func(ctx context.Context)

// Timer is a processor of handle functor with retry duration
type Timer interface {
	Start(ctx context.Context, handle handle)
}

type timer struct {
	retry time.Duration
}

func (t *timer) Start(ctx context.Context, handle handle) {
	timer := time.NewTimer(t.retry)

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return

		case <-timer.C:
			handle(ctx)
			timer.Reset(t.retry)
		}
	}
}

// NewTimer creates new timer processor with retry duration
func NewTimer(retry time.Duration) Timer {
	return &timer{
		retry: retry,
	}
}
