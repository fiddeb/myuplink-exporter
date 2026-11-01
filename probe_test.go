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
	pointFailure, alarmFailure, systemFailure, cleared, noTimestamp, invalidAlarm := false, false, false, false, false, false
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
			if invalidAlarm {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, "<html>Maintenance</html>")
				return
			}
			if alarmFailure {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if cleared {
				fmt.Fprint(w, `{"numItems":0,"notifications":[]}`)
				return
			}
			fmt.Fprint(w, `{"numItems":2,"notifications":[{"id":"a","deviceId":"good","alarmNumber":42,"severity":1},{"id":"b","deviceId":"good","alarmNumber":42,"severity":1}]}`)
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
	previousEvents := events
	t.Cleanup(func() {
		Opts, logger, myuplinkClient, globalCache = previousOpts, previousLogger, previousClient, previousCache
		events = previousEvents
	})
	events = newEventTracker()
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
	alarmFailure, invalidAlarm = false, true
	body = probe()
	contains(body, "myuplink_probe_success 0")
	if strings.Contains(body, "myuplink_system_active_alarms{") || len(events.alarms["s"]) != 2 {
		t.Fatal("invalid alarm response must not report zero or resolve known alarms")
	}
	invalidAlarm, cleared = false, true
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

func TestEcoZenithUnavailableMeasurements(t *testing.T) {
	previousTotals := totalParamCache.deviceParamMap
	totalParamCache.Init()
	t.Cleanup(func() { totalParamCache.deviceParamMap = previousTotals })
	value := -32768.0
	product := "CTC EcoZenith i360"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			fmt.Fprint(w, `{"access_token":"test","expires_in":3600}`)
		case "/v2/systems/me":
			fmt.Fprintf(w, `{"numItems":1,"systems":[{"systemId":"s","devices":[{"id":"d","connectionState":"Connected","product":{"name":%q}}]}]}`, product)
		case "/v2/devices/d/points":
			fmt.Fprintf(w, `[{"parameterId":"62015","value":%g,"timestamp":"2026-11-01T07:23:03Z","enumValues":[{"value":"-32768","text":"invalid"}]},{"parameterId":"62027","value":46}]`, value)
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
	Opts.MyUplink.CollectAlarms = false
	Opts.MyUplink.Device.AllowedConnectionStates = []string{"Connected"}
	Opts.MyUplink.Device.CalcTotalParameters = []string{"62015"}
	myuplinkClient = myuplink.NewClient(logger)
	myuplinkClient.SetApiUrl(server.URL)
	myuplinkClient.SetAuth("test", "test")
	if err := myuplinkClient.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	globalCache = cache.New(time.Minute, 0)
	probe := func() string {
		globalCache.Flush()
		recorder := httptest.NewRecorder()
		myuplinkProbe(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status %d", recorder.Code)
		}
		return recorder.Body.String()
	}
	for _, v := range []float64{-32768, 36, -32768, 35} {
		value = v
		body := probe()
		if !strings.Contains(body, "myuplink_probe_success 1") || !strings.Contains(body, `parameterID="62027"`) {
			t.Fatalf("invalid point must not fail collection or suppress other points: %s", body)
		}
		present := strings.Contains(body, `parameterID="62015"`)
		if present != (v != -32768) {
			t.Fatalf("unexpected measurement availability for %g: %s", v, body)
		}
	}
	// The device-specific sentinel must not suppress legitimate negative values
	// or the same number in an unrelated product's parameter namespace.
	Opts.MyUplink.Device.CalcTotalParameters = nil
	for _, tc := range []struct {
		product string
		value   float64
	}{
		{"CTC EcoZenith i360", -60}, {"Other product", -32768}, {"CTC EcoZenith i360", 32768},
	} {
		product, value = tc.product, tc.value
		if body := probe(); !strings.Contains(body, `parameterID="62015"`) {
			t.Fatalf("valid value suppressed: %s", body)
		}
	}
}
