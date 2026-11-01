package myuplink

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestRetryAfterAndCancellation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	if retryDelay("12", now) != 12*time.Second || retryDelay(now.Add(time.Minute).Format(http.TimeFormat), now) != time.Minute || retryDelay("invalid", now) != time.Minute {
		t.Fatal("Retry-After parsing")
	}
	calls := 0
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":"slow_down"}`)
	})
	if err := client.Connect(context.Background()); err == nil {
		t.Fatal("expected 429")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := client.Connect(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("cooldown bypassed: %d calls", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestQueuedRequestCancellation(t *testing.T) {
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	gate := &requestGate{mu: make(contextMutex, 1), base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/first" {
			close(started)
			<-release
		}
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})}
	first, _ := http.NewRequest(http.MethodGet, "http://example/first", nil)
	go func() {
		defer close(done)
		resp, err := gate.RoundTrip(first)
		if err != nil {
			t.Error(err)
			return
		}
		_ = resp.Body.Close()
	}()
	<-started
	defer func() { close(release); <-done }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	second, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example/second", nil)
	result := make(chan error, 1)
	go func() { _, err := gate.RoundTrip(second); result <- err }()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued request did not cancel while the first request was in flight")
	}
}

func TestTokenRefreshWaitCancellation(t *testing.T) {
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test","expires_in":3600}`)
	})
	go func() {
		defer close(done)
		if err := client.Connect(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	<-started
	defer func() { close(release); <-done }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := client.GetSystems(ctx); result <- err }()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not cancel while waiting for token refresh")
	}
}

func TestRequestSpacing(t *testing.T) {
	var previous time.Time
	gate := &requestGate{mu: make(contextMutex, 1), interval: 20 * time.Millisecond,
		base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			now := time.Now()
			if !previous.IsZero() && now.Sub(previous) < 20*time.Millisecond {
				t.Error("request spacing bypassed")
			}
			previous = now
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
		})}
	for range 3 {
		req, _ := http.NewRequest(http.MethodGet, "http://example", nil)
		resp, err := gate.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}
