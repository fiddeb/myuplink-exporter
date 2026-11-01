package myuplink

import "context"

// contextMutex allows a queued request to leave when its deadline expires.
// Initialize it with make(contextMutex, 1).
type contextMutex chan struct{}

func (m contextMutex) Lock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case m <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m contextMutex) Unlock() { <-m }
