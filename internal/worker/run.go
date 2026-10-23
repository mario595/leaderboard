package worker

import (
	"context"
	"time"
)

// Run processes due events until ctx is cancelled, then returns nil. It
// drains every due event, oldest first, then waits for a wake, the next poll
// or cancellation. Cancellation stops it between events: the event in
// progress runs with a context that ignores it, so shutdown never leaves an
// event half done. A queue error is logged and retried at the next wake or
// poll.
func (w *Worker) Run(ctx context.Context) error {
	for {
		for {
			if ctx.Err() != nil {
				return nil
			}
			e, found, err := w.events.NextDue(ctx)

			if err != nil {
				w.logger.Error("next due event", "error", err)
				break
			}

			if !found {
				break
			}

			w.handle(context.WithoutCancel(ctx), e)

		}
		select {
		case <-ctx.Done():
			return nil
		case <-w.wake:
		case <-time.After(w.pollInterval):
		}
	}

}
