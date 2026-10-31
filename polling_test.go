package main

import (
	"bytes"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/webdevops/go-common/log/slogger"
)

func TestChangeEvents(t *testing.T) {
	old := logger
	defer func() { logger = old }()
	var out bytes.Buffer
	logger = slogger.NewCliLogger(&out, slogger.WithFormat(slogger.FormatMode("json")))
	tracker := newEventTracker()
	tracker.point("s", "d", "p", "Heating", time.Unix(1, 0))
	tracker.point("s", "d", "p", "Heating", time.Unix(2, 0))
	tracker.point("s", "d", "p", "", time.Unix(3, 0))
	tracker.alarmSet("s", map[string]bool{"a": true})
	tracker.alarmSet("s", map[string]bool{"a": true})
	tracker.alarmSet("s", map[string]bool{})
	if strings.Count(out.String(), "point_observed") != 1 || strings.Count(out.String(), "point_changed") != 1 || strings.Count(out.String(), "alarm_observed") != 1 || strings.Count(out.String(), "alarm_no_longer_active") != 1 {
		t.Fatal(out.String())
	}
}

func TestSnapshotScrapes(t *testing.T) {
	p := &pollSnapshot{lastSuccess: time.Unix(0, 0)}
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	p.ready = true
	p.body = []byte("myuplink_probe_success 1\n")
	for range 3 {
		w = httptest.NewRecorder()
		p.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))
		body, _ := io.ReadAll(w.Body)
		if !strings.Contains(string(body), "myuplink_poll_last_success_timestamp_seconds 0") {
			t.Fatal(string(body))
		}
	}
}

func TestNumericStringFiltering(t *testing.T) {
	oldLogger, oldOpts := logger, Opts
	defer func() { logger, Opts = oldLogger, oldOpts }()
	var out bytes.Buffer
	logger = slogger.NewCliLogger(&out, slogger.WithFormat(slogger.FormatMode("json")))
	tracker := newEventTracker()
	for _, value := range []string{"6°C", "60%", "0kWh", "-42.3", "9.1", "15.7bar"} {
		tracker.point("s", "d", "p", value, time.Now())
	}
	if out.Len() != 0 {
		t.Fatal("numeric values logged by default")
	}
	tracker.point("s", "d", "state", "Normal", time.Now())
	if !strings.Contains(out.String(), "observed_at") || !strings.Contains(out.String(), "point_observed") {
		t.Fatal(out.String())
	}
	Opts.MyUplink.LogNumericStrings = true
	tracker.point("s", "d", "p", "6°C", time.Now())
	if !strings.Contains(out.String(), "6°C") {
		t.Fatal("numeric opt-in failed")
	}
}
