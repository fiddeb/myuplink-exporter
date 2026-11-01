package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/webdevops/myuplink-exporter/myuplink"
)

type (
	TotalParameterCache struct {
		lock           sync.Mutex
		deviceParamMap map[string]*TotalParameterMetric
	}

	TotalParameterMetric struct {
		updated time.Time
		value   float64
	}
)

var (
	totalParamCache TotalParameterCache
)

func (c *TotalParameterCache) Init() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.deviceParamMap = map[string]*TotalParameterMetric{}
}

func (c *TotalParameterCache) getParameterValue(deviceID string, parameterID string, devicePoint myuplink.SystemDevicePoint) float64 {
	key := fmt.Sprintf(
		"%s:%s",
		deviceID,
		parameterID,
	)

	c.lock.Lock()
	defer c.lock.Unlock()
	// Lookup and timestamp deduplication must be atomic across concurrent probes.
	if _, exists := c.deviceParamMap[key]; !exists {
		c.deviceParamMap[key] = &TotalParameterMetric{
			updated: devicePoint.Timestamp,
			value:   *devicePoint.Value,
		}
	}

	// increase check
	if c.deviceParamMap[key].updated != devicePoint.Timestamp {
		c.deviceParamMap[key].updated = devicePoint.Timestamp
		c.deviceParamMap[key].value += *devicePoint.Value
	}

	return c.deviceParamMap[key].value
}
