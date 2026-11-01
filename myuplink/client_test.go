package myuplink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/webdevops/go-common/log/slogger"
	resty "resty.dev/v3"
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

func TestRejectedTokenRefresh(t *testing.T) {
	for _, rejectReplacement := range []bool{false, true} {
		t.Run(fmt.Sprintf("reject_replacement=%t", rejectReplacement), func(t *testing.T) {
			var tokens, requests atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/oauth/token" {
					tokens.Add(1)
					fmt.Fprint(w, `{"access_token":"replacement","expires_in":3600}`)
					return
				}
				requests.Add(1)
				if rejectReplacement || r.Header.Get("Authorization") != "Bearer replacement" {
					w.WriteHeader(http.StatusUnauthorized)
					fmt.Fprint(w, `{"errorCode":"Unauthorized"}`)
					return
				}
				fmt.Fprint(w, `{"systems":[]}`)
			})
			client.token = &AuthToken{AccessToken: "revoked", expiry: time.Now().Add(time.Hour)}
			_, err := client.GetSystems(context.Background())
			if rejectReplacement {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.HTTPStatusCode != http.StatusUnauthorized {
					t.Fatalf("expected bounded 401 failure, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tokens.Load() != 1 || requests.Load() != 2 {
				t.Fatalf("got %d refreshes and %d API attempts, want 1 and 2", tokens.Load(), requests.Load())
			}
		})
	}
}

func TestConcurrentRejectedTokenRefresh(t *testing.T) {
	var tokens atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/token" {
			tokens.Add(1)
			fmt.Fprint(w, `{"access_token":"replacement","expires_in":3600}`)
		} else if r.Header.Get("Authorization") == "Bearer replacement" {
			fmt.Fprint(w, `{"systems":[]}`)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
	client.token = &AuthToken{AccessToken: "revoked", expiry: time.Now().Add(time.Hour)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	blocked, release := context.WithCancel(ctx)
	oldResponseReady := make(chan struct{})
	var delayed atomic.Bool
	client.http.AddResponseMiddleware(func(_ *resty.Client, response *resty.Response) error {
		if response.StatusCode() == http.StatusUnauthorized && delayed.CompareAndSwap(false, true) {
			// Hold the first stale 401 until another caller has obtained a replacement.
			close(oldResponseReady)
			<-blocked.Done()
		}
		return nil
	})
	first := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Go(func() {
		_, err := client.GetSystems(ctx)
		first <- err
	})
	defer wg.Wait()
	defer release()
	select {
	case <-oldResponseReady:
	case <-ctx.Done():
		t.Fatal("first request did not reach its 401 response")
	}
	if _, err := client.GetSystems(ctx); err != nil {
		t.Fatal(err)
	}
	release()
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if tokens.Load() != 1 {
		t.Fatalf("got %d token refreshes, want 1", tokens.Load())
	}
}

func TestJSONResponseContract(t *testing.T) {
	collectors := []struct {
		name, body string
		collect    func(*Client) error
	}{
		{"systems", `{"numItems":1,"systems":[{"systemId":"s"}]}`, func(c *Client) error {
			result, err := c.GetSystems(context.Background())
			if err == nil && len(result.Systems) != 1 {
				return fmt.Errorf("systems were not decoded")
			}
			return err
		}},
		{"alarms", `{"numItems":1,"notifications":[{"id":"alarm"}]}`, func(c *Client) error {
			result, err := c.GetSystemActiveAlarms(context.Background(), "s")
			if err == nil && len(result) != 1 {
				return fmt.Errorf("alarms were not decoded")
			}
			return err
		}},
		{"points", `[{"parameterId":"p","value":42}]`, func(c *Client) error {
			result, err := c.GetSystemDevicePoints(context.Background(), "d")
			if err == nil && len(*result) != 1 {
				return fmt.Errorf("points were not decoded")
			}
			return err
		}},
		{"firmware", `{"currentFwVersion":"1"}`, func(c *Client) error {
			result, err := c.GetSystemDeviceFirmware(context.Background(), "d")
			if err == nil && result.CurrentFwVersion != "1" {
				return fmt.Errorf("firmware was not decoded")
			}
			return err
		}},
	}
	for _, collector := range collectors {
		for _, contentType := range []string{"application/json", "text/plain", ""} {
			t.Run(collector.name+"/"+contentType, func(t *testing.T) {
				client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Accept") != "application/json" {
						t.Error("JSON was not requested")
					}
					w.Header()["Content-Type"] = []string{contentType}
					fmt.Fprint(w, collector.body)
				})
				client.token = &AuthToken{AccessToken: "valid", expiry: time.Now().Add(time.Hour)}
				if err := collector.collect(client); err != nil {
					t.Fatal(err)
				}
			})
		}
		for _, body := range []string{"<html>private upstream detail</html>", "", "null", collector.body + " garbage"} {
			t.Run(collector.name+"/invalid/"+body, func(t *testing.T) {
				client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, body)
				})
				client.token = &AuthToken{AccessToken: "valid", expiry: time.Now().Add(time.Hour)}
				err := collector.collect(client)
				if err == nil || !strings.Contains(err.Error(), "invalid JSON response") || strings.Contains(err.Error(), "private") {
					t.Fatalf("expected sanitized response error, got %v", err)
				}
			})
		}
	}
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
