package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/webdevops/go-common/log/slogger"

	"github.com/webdevops/myuplink-exporter/myuplink"
)

const (
	DefaultTimeout = 30
)

func myuplinkProbe(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64

	contextLogger := buildContextLoggerFromRequest(r)
	registry := prometheus.NewRegistry()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, DefaultTimeout)
	if err != nil {
		logCollectionError(contextLogger, err)
		http.Error(w, fmt.Sprintf("failed to parse timeout from Prometheus header: %s", err), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	// Keep system status and data points fresh on the same cache schedule.
	cacheTime := time.Minute

	if v := r.URL.Query().Get("cache"); v != "" {
		cacheTime, err = time.ParseDuration(v)
		if err != nil || cacheTime <= 0 {
			http.Error(w, fmt.Sprintf("cache must be a positive duration: %q", v), http.StatusBadRequest)
			return
		}
	}

	metrics := NewMyUplinkMetrics(registry)
	success := true
	defer func() {
		metrics.probeSuccess.Set(boolFloat(success))
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	}()

	systemList, err := cacheResultWithDuration(
		"systems", cacheTime,
		func() (interface{}, error) {
			return myuplinkClient.GetSystems(ctx)
		},
	)
	if err != nil {
		logCollectionError(contextLogger, err)
		success = false
		return
	}

	for _, system := range systemList.(*myuplink.ResultSystems).Systems {
		metrics.systemHasAlarm.WithLabelValues(system.SystemID).Set(boolFloat(system.HasAlarm))
		if Opts.MyUplink.CollectAlarms {
			alarms, alarmErr := cacheResultWithDuration("alarms:"+system.SystemID, cacheTime, func() (interface{}, error) {
				return myuplinkClient.GetSystemActiveAlarms(ctx, system.SystemID)
			})
			if alarmErr != nil {
				logCollectionError(contextLogger, alarmErr)
				success = false
			} else {
				list := alarms.([]myuplink.Alarm)
				if events != nil {
					current := make(map[string]bool)
					for _, a := range list {
						current[a.ID] = true
					}
					events.alarmSet(system.SystemID, current)
				}
				metrics.systemActiveAlarms.WithLabelValues(system.SystemID).Set(float64(len(list)))
				for _, alarm := range list {
					metrics.systemAlarm.WithLabelValues(system.SystemID, alarm.DeviceID, strconv.Itoa(alarm.AlarmNumber), strconv.Itoa(alarm.Severity)).Add(1)
				}
			}
		}
		metrics.system.With(prometheus.Labels{
			"systemID":   system.SystemID,
			"systemName": clearText(system.Name),
			"country":    clearText(system.Country),
		}).Set(1)

		for _, device := range system.Devices {
			metrics.systemDevice.With(prometheus.Labels{
				"systemID":        system.SystemID,
				"deviceID":        device.ID,
				"deviceName":      clearText(device.Product.Name),
				"serialNumber":    clearText(device.Product.SerialNumber),
				"connectionState": clearText(device.ConnectionState),
				"firmwareVersion": clearText(device.CurrentFwVersion),
			}).Set(1)
			metrics.systemDeviceConnected.WithLabelValues(system.SystemID, device.ID).Set(boolFloat(strings.EqualFold(device.ConnectionState, "Connected")))

			if !device.IsConnectionStateAllowed(Opts.MyUplink.Device.AllowedConnectionStates) {
				contextLogger.Warnf(`ignoring system "%s" device "%s", connection state is "%s"`, system.Name, device.ID, device.ConnectionState)
				continue
			}

			devicePoints, err := cacheResultWithDuration(
				fmt.Sprintf("devicePoints:%s", device.ID),
				cacheTime,
				func() (interface{}, error) {
					return myuplinkClient.GetSystemDevicePoints(ctx, device.ID)
				},
			)
			if err != nil {
				logCollectionError(contextLogger, err)
				success = false
				continue
			}

			for _, devicePoint := range *devicePoints.(*myuplink.SystemDevicePoints) {
				if events != nil && devicePoint.StrVal != nil {
					events.point(system.SystemID, device.ID, devicePoint.ParameterID, *devicePoint.StrVal, devicePoint.Timestamp)
				}
				if !devicePoint.Timestamp.IsZero() {
					metrics.pointTimestamp.WithLabelValues(system.SystemID, device.ID, devicePoint.ParameterID).Set(float64(devicePoint.Timestamp.UnixNano()) / 1e9)
				}
				if devicePoint.Value != nil {
					metrics.systemDevicePoint.With(prometheus.Labels{
						"systemID":      system.SystemID,
						"deviceID":      device.ID,
						"category":      clearText(devicePoint.Category),
						"parameterID":   devicePoint.ParameterID,
						"parameterName": clearText(devicePoint.ParameterName),
						"parameterUnit": clearText(devicePoint.ParameterUnit),
					}).Set(*devicePoint.Value)

					// enum translation
					enumValue := fmt.Sprintf("%d", int64(*devicePoint.Value))
					for _, enumVal := range devicePoint.EnumValues {
						enumMetricVal := float64(0)
						if enumVal.Value == enumValue {
							enumMetricVal = 1
						}

						metrics.systemDevicePointEnum.With(prometheus.Labels{
							"systemID":      system.SystemID,
							"deviceID":      device.ID,
							"category":      clearText(devicePoint.Category),
							"parameterID":   devicePoint.ParameterID,
							"parameterName": clearText(devicePoint.ParameterName),
							"parameterUnit": clearText(devicePoint.ParameterUnit),
							"valueText":     clearText(enumVal.Text),
						}).Set(enumMetricVal)
					}

					// total values (counters)
					for _, totalParameterID := range Opts.MyUplink.Device.CalcTotalParameters {
						if strings.EqualFold(devicePoint.ParameterID, totalParameterID) {
							metrics.systemDevicePointTotal.With(prometheus.Labels{
								"systemID":      system.SystemID,
								"deviceID":      device.ID,
								"category":      clearText(devicePoint.Category),
								"parameterID":   devicePoint.ParameterID,
								"parameterName": clearText(devicePoint.ParameterName),
								"parameterUnit": clearText(devicePoint.ParameterUnit),
							}).Set(totalParamCache.getParameterValue(device.ID, devicePoint.ParameterID, devicePoint))
						}
					}
				}
			}
		}
	}
}

func buildContextLoggerFromRequest(r *http.Request) *slogger.Logger {
	return logger.With(
		slog.Group(
			"request",
			slog.String("host", r.Host),
			slog.String("path", r.URL.Path),
		),
	)
}

func getPrometheusTimeout(r *http.Request, defaultTimeout float64) (timeout float64, err error) {
	// If a timeout is configured via the Prometheus header, add it to the request.
	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		timeout, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
	}
	if math.IsNaN(timeout) || math.IsInf(timeout, 0) || timeout < 0 || timeout > float64(math.MaxInt64)/float64(time.Second) {
		return 0, fmt.Errorf("invalid scrape timeout")
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return
}

func cacheResultWithDuration(cacheKey string, cacheTime time.Duration, callback func() (interface{}, error)) (interface{}, error) {
	if val, ok := globalCache.Get(cacheKey); ok {
		return val, nil
	}

	ret, err := callback()
	if err != nil {
		return nil, err
	}

	globalCache.Set(cacheKey, ret, cacheTime)

	return ret, nil
}

func clearText(val string) string {
	// remove soft hyphen
	val = strings.ReplaceAll(val, "\u00AD", "")
	// remove possible space chars
	val = strings.TrimSpace(val)

	return val
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

// Use errors.As so wrapped authentication errors retain structured fields.
func logCollectionError(logger *slogger.Logger, err error) {
	var apiErr *myuplink.APIError
	var oauthErr *myuplink.OAuthError
	switch {
	case errors.As(err, &apiErr):
		logger.Error("myUplink API request failed", slog.Any("error", apiErr))
	case errors.As(err, &oauthErr):
		logger.Error("myUplink authentication failed", slog.Any("error", oauthErr))
	default:
		logger.Error(err.Error())
	}
}
