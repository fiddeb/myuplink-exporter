package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type (
	MyUplinkMetrics struct {
		probeSuccess           prometheus.Gauge
		systemHasAlarm         *prometheus.GaugeVec
		systemActiveAlarms     *prometheus.GaugeVec
		systemAlarm            *prometheus.GaugeVec
		systemDeviceConnected  *prometheus.GaugeVec
		pointTimestamp         *prometheus.GaugeVec
		system                 *prometheus.GaugeVec
		systemDevice           *prometheus.GaugeVec
		systemDevicePoint      *prometheus.GaugeVec
		systemDevicePointEnum  *prometheus.GaugeVec
		systemDevicePointTotal *prometheus.GaugeVec
	}
)

func NewMyUplinkMetrics(registry *prometheus.Registry) *MyUplinkMetrics {
	metrics := &MyUplinkMetrics{}

	pointLabels := []string{"systemID", "deviceID", "category", "parameterID", "parameterName", "parameterUnit"}
	metrics.probeSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "myuplink_probe_success", Help: "Whether all requested myUplink collections succeeded",
	})
	metrics.systemHasAlarm = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "myuplink_system_has_alarm", Help: "Whether the system reports an active alarm",
	}, []string{"systemID"})
	metrics.systemActiveAlarms = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "myuplink_system_active_alarms", Help: "Number of active alarms returned by the API",
	}, []string{"systemID"})
	metrics.systemAlarm = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "myuplink_system_alarm_active", Help: "Number of active alarms grouped by device, alarm number and severity",
	}, []string{"systemID", "deviceID", "alarmNumber", "severity"})
	metrics.systemDeviceConnected = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "myuplink_system_device_connected", Help: "Whether the device is connected to myUplink",
	}, []string{"systemID", "deviceID"})
	metrics.pointTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "myuplink_system_device_point_timestamp_seconds", Help: "Source timestamp of the data point as Unix seconds",
	}, []string{"systemID", "deviceID", "parameterID"})
	registry.MustRegister(metrics.probeSuccess, metrics.systemHasAlarm, metrics.systemActiveAlarms,
		metrics.systemAlarm, metrics.systemDeviceConnected, metrics.pointTimestamp)

	metrics.system = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myuplink_system_info",
			Help: "myUplink system information",
		},
		[]string{"systemID", "systemName", "country"},
	)
	registry.MustRegister(metrics.system)

	metrics.systemDevice = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myuplink_system_device_info",
			Help: "myUplink system device information",
		},
		[]string{"systemID", "deviceID", "deviceName", "serialNumber", "connectionState", "firmwareVersion"},
	)
	registry.MustRegister(metrics.systemDevice)

	metrics.systemDevicePoint = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myuplink_system_device_point",
			Help: "myUplink device metric point",
		},
		pointLabels,
	)
	registry.MustRegister(metrics.systemDevicePoint)

	metrics.systemDevicePointEnum = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myuplink_system_device_point_enum",
			Help: "myUplink device metric point enum value",
		},
		append(pointLabels, "valueText"),
	)
	registry.MustRegister(metrics.systemDevicePointEnum)

	metrics.systemDevicePointTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myuplink_system_device_point_total",
			Help: "myUplink device metric point total",
		},
		pointLabels,
	)
	registry.MustRegister(metrics.systemDevicePointTotal)

	return metrics
}
