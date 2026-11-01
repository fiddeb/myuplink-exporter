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
