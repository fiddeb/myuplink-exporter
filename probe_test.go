package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/webdevops/myuplink-exporter/myuplink"
)

func TestProbeMonitoring(t *testing.T) {
	alarmCalls := 0
	pointFailure, alarmFailure, systemFailure, cleared, noTimestamp := false, false, false, false, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			fmt.Fprint(w, `{"access_token":"test","expires_in":3600}`)
		case "/v2/systems/me":
			if systemFailure {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			fmt.Fprintf(w, `{"numItems":1,"systems":[{"systemId":"s","hasAlarm":%t,"devices":[{"id":"offline","connectionState":"Disconnected"},{"id":"bad","connectionState":"Connected"},{"id":"good","connectionState":"Connected"}]}]}`, !cleared)
		case "/v2/systems/s/notifications/active":
			alarmCalls++
			if alarmFailure {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if cleared {
				fmt.Fprint(w, `{"numItems":0,"notifications":[]}`)
				return
			}
			fmt.Fprint(w, `{"numItems":2,"notifications":[{"deviceId":"good","alarmNumber":42,"severity":1},{"deviceId":"good","alarmNumber":42,"severity":1}]}`)
		case "/v2/devices/bad/points":
			if pointFailure {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			fmt.Fprint(w, `[]`)
		case "/v2/devices/good/points":
			if noTimestamp {
				fmt.Fprint(w, `[{"parameterId":"p","value":12.5}]`)
				return
			}
			fmt.Fprint(w, `[{"parameterId":"p","value":12.5,"timestamp":"2026-01-01T00:00:00Z","enumValues":[{"value":"12.50","text":"fractional"},{"value":"12","text":"integer"}]}]`)
		default:
			t.Errorf("unexpected API call: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	previousOpts, previousLogger, previousClient, previousCache := Opts, logger, myuplinkClient, globalCache
	t.Cleanup(func() {
		Opts, logger, myuplinkClient, globalCache = previousOpts, previousLogger, previousClient, previousCache
	})
	Opts.Logger.Level = "error"
	initLogger()
	Opts.MyUplink.Device.AllowedConnectionStates = []string{"Connected"}
	myuplinkClient = myuplink.NewClient(logger)
	myuplinkClient.SetApiUrl(server.URL)
	myuplinkClient.SetAuth("test", "test")
	if err := myuplinkClient.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	globalCache = cache.New(time.Minute, 0)
	probe := func() string {
		t.Helper()
		globalCache.Flush()
		recorder := httptest.NewRecorder()
		myuplinkProbe(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
		}
		return recorder.Body.String()
	}
	contains := func(body, want string) {
		t.Helper()
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	body := probe()
	contains(body, `myuplink_system_device_connected{deviceID="offline",systemID="s"} 0`)
	contains(body, `myuplink_system_device_connected{deviceID="good",systemID="s"} 1`)
	contains(body, `myuplink_system_has_alarm{systemID="s"} 1`)
	contains(body, `myuplink_system_device_point_timestamp_seconds{deviceID="good",parameterID="p",systemID="s"} 1.7672256e+09`)
	contains(body, "myuplink_probe_success 1")
	contains(body, `myuplink_system_device_point_enum{category="",deviceID="good",parameterID="p",parameterName="",parameterUnit="",systemID="s",valueText="fractional"} 1`)
	contains(body, `myuplink_system_device_point_enum{category="",deviceID="good",parameterID="p",parameterName="",parameterUnit="",systemID="s",valueText="integer"} 0`)
	if alarmCalls != 0 {
		t.Fatal("alarm detail collection must be opt-in")
	}
	Opts.MyUplink.CollectAlarms = true
	body = probe()
	contains(body, `myuplink_system_active_alarms{systemID="s"} 2`)
	contains(body, `myuplink_system_alarm_active{alarmNumber="42",deviceID="good",severity="1",systemID="s"} 2`)
	pointFailure = true
	body = probe()
	contains(body, "myuplink_probe_success 0")
	contains(body, `myuplink_system_device_point{category="",deviceID="good",parameterID="p",parameterName="",parameterUnit="",systemID="s"} 12.5`)
	pointFailure, alarmFailure = false, true
	body = probe()
	contains(body, "myuplink_probe_success 0")
	if strings.Contains(body, "myuplink_system_active_alarms{") {
		t.Fatal("failed alarm request must not report zero alarms")
	}
	alarmFailure, cleared = false, true
	body = probe()
	contains(body, `myuplink_system_has_alarm{systemID="s"} 0`)
	contains(body, `myuplink_system_active_alarms{systemID="s"} 0`)
	if strings.Contains(body, "myuplink_system_alarm_active{") {
		t.Fatal("cleared alarms must disappear")
	}
	noTimestamp = true
	body = probe()
	if strings.Contains(body, "myuplink_system_device_point_timestamp_seconds{") {
		t.Fatal("missing source timestamp must be omitted")
	}
	systemFailure = true
	contains(probe(), "myuplink_probe_success 0")
}

func TestInvalidProbeParameters(t *testing.T) {
	old, oldOpts := logger, Opts
	Opts.Logger.Level = "error"
	initLogger()
	t.Cleanup(func() { logger, Opts = old, oldOpts })
	for _, value := range []string{"-1", "NaN", "+Inf", "1e100", "invalid"} {
		r := httptest.NewRequest(http.MethodGet, "/probe", nil)
		r.Header.Set("X-Prometheus-Scrape-Timeout-Seconds", value)
		if _, err := getPrometheusTimeout(r, 30); err == nil {
			t.Errorf("accepted timeout %s", value)
		}
	}
	for _, value := range []string{"0s", "-1s", "invalid"} {
		recorder := httptest.NewRecorder()
		myuplinkProbe(recorder, httptest.NewRequest(http.MethodGet, "/probe?cache="+value, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("accepted cache %s", value)
		}
	}
}
