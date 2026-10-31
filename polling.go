package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type pollSnapshot struct {
	mu          sync.RWMutex
	body        []byte
	ready       bool
	lastSuccess time.Time
}

func (p *pollSnapshot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.ready {
		http.Error(w, "first poll has not completed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write(p.body)
	_, _ = fmt.Fprintf(w, "myuplink_poll_last_success_timestamp_seconds %d\n", p.lastSuccess.Unix())
}

type pollWriter struct {
	bytes.Buffer
	header http.Header
}

func (w *pollWriter) Header() http.Header { return w.header }
func (w *pollWriter) WriteHeader(_ int)   {}
func (p *pollSnapshot) run(ctx context.Context, interval, timeout time.Duration) {
	for {
		pollCtx, cancel := context.WithTimeout(ctx, timeout)
		req, _ := http.NewRequestWithContext(pollCtx, http.MethodGet, "http://localhost/probe", nil)
		req.Header.Set("X-Prometheus-Scrape-Timeout-Seconds", strconv.FormatFloat(timeout.Seconds(), 'f', -1, 64))
		// Only the poller invokes the API collector in background mode.
		globalCache.Flush()
		out := &pollWriter{header: make(http.Header)}
		myuplinkProbe(out, req)
		cancel()
		p.mu.Lock()
		p.body = append([]byte(nil), out.Bytes()...)
		p.ready = true
		if strings.Contains(out.String(), "myuplink_probe_success 1\n") {
			p.lastSuccess = time.Now()
		}
		p.mu.Unlock()
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

type eventTracker struct {
	strings map[string]string
	alarms  map[string]map[string]bool
}

func newEventTracker() *eventTracker {
	return &eventTracker{strings: make(map[string]string), alarms: make(map[string]map[string]bool)}
}

var events *eventTracker

func (e *eventTracker) point(system, device, param, value string, timestamp time.Time) {
	key := system + "/" + device + "/" + param
	old, exists := e.strings[key]
	if exists && old == value {
		return
	}
	kind := "changed"
	if !exists {
		kind = "observed"
	}
	logger.Info("myUplink string point", slog.String("event", "point_"+kind), slog.String("system_id", system), slog.String("device_id", device), slog.String("parameter_id", param), slog.String("value", value), slog.Time("source_timestamp", timestamp))
	e.strings[key] = value
}
func (e *eventTracker) alarmSet(system string, current map[string]bool) {
	previous, known := e.alarms[system]
	for id := range current {
		if !previous[id] {
			kind := "alarm_active"
			if !known {
				kind = "alarm_observed"
			}
			logger.Info("myUplink alarm", slog.String("event", kind), slog.String("system_id", system), slog.String("alarm_id", id))
		}
	}
	for id := range previous {
		if !current[id] {
			logger.Info("myUplink alarm", slog.String("event", "alarm_no_longer_active"), slog.String("system_id", system), slog.String("alarm_id", id))
		}
	}
	e.alarms[system] = current
}
