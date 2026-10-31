package myuplink

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/webdevops/go-common/log/slogger"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClient(slogger.NewCliLogger(io.Discard))
	client.SetApiUrl(server.URL)
	client.SetAuth("client", "secret")
	client.http.SetRetryCount(0)
	t.Cleanup(func() { _ = client.http.Close() })
	return client
}

func TestPagination(t *testing.T) {
	var systemPages, alarmPages int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := 1
		if r.URL.Query().Get("page") == "2" {
			page = 2
		}
		switch r.URL.Path {
		case "/oauth/token":
			fmt.Fprint(w, `{"access_token":"test","expires_in":3600}`)
		case "/v2/systems/me":
			systemPages++
			fmt.Fprintf(w, `{"itemsPerPage":1,"numItems":2,"systems":[{"systemId":"s%d"}]}`, page)
		case "/v2/systems/s1/notifications/active":
			alarmPages++
			fmt.Fprintf(w, `{"itemsPerPage":1,"numItems":2,"notifications":[{"alarmNumber":%d}]}`, page)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	systems, err := client.GetSystems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(systems.Systems) != 2 || systems.Systems[1].SystemID != "s2" || systemPages != 2 {
		t.Fatalf("incomplete systems: %+v", systems)
	}
	alarms, err := client.GetSystemActiveAlarms(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(alarms) != 2 || alarms[1].AlarmNumber != 2 || alarmPages != 2 {
		t.Fatalf("incomplete alarms: %+v", alarms)
	}
}

func TestTokenRefreshFailureAndRecovery(t *testing.T) {
	fail := true
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/token" {
			if fail {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"access_token":"new","expires_in":3600}`)
			return
		}
		fmt.Fprint(w, `{"systems":[]}`)
	})
	client.token = &AuthToken{AccessToken: "expired", expiry: time.Now().Add(-time.Second)}
	if _, err := client.GetSystems(context.Background()); err == nil {
		t.Fatal("refresh failure must return an error")
	}
	fail = false
	if _, err := client.GetSystems(context.Background()); err != nil {
		t.Fatalf("must recover: %v", err)
	}
}

func TestConcurrentTokenRefresh(t *testing.T) {
	var tokens atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/token" {
			tokens.Add(1)
			fmt.Fprint(w, `{"access_token":"test","expires_in":3600}`)
		} else {
			fmt.Fprint(w, `{"systems":[]}`)
		}
	})
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := client.GetSystems(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if tokens.Load() != 1 {
		t.Fatalf("got %d token requests, want 1", tokens.Load())
	}
}
