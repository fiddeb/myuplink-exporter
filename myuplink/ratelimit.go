package myuplink

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// requestGate serializes API calls across concurrent users of one client.
// Quotas can differ by account: configure the minimum spacing accordingly.
type requestGate struct {
	base     http.RoundTripper
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func waitUntil(ctx context.Context, until time.Time) error {
	timer := time.NewTimer(time.Until(until))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func retryDelay(header string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(header, 10, 32); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(header); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return time.Minute
}
func (g *requestGate) RoundTrip(req *http.Request) (*http.Response, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := waitUntil(req.Context(), g.next); err != nil {
		return nil, err
	}
	resp, err := g.base.RoundTrip(req)
	now := time.Now()
	g.next = now.Add(g.interval)
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		delay := retryDelay(resp.Header.Get("Retry-After"), now)
		if delay < g.interval {
			delay = g.interval
		}
		g.next = now.Add(delay)
	}
	return resp, err
}
func (c *Client) SetRequestInterval(interval time.Duration) {
	c.gate.mu.Lock()
	defer c.gate.mu.Unlock()
	c.gate.interval = interval
}
