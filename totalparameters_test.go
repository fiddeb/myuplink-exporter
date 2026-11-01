package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/webdevops/myuplink-exporter/myuplink"
)

func TestConcurrentTotalParameters(t *testing.T) {
	c := &TotalParameterCache{}
	c.Init()
	value := 1.0
	for timestamp := int64(1); timestamp <= 2; timestamp++ {
		point := myuplink.SystemDevicePoint{Value: &value, Timestamp: time.Unix(timestamp, 0)}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for range 32 {
			wg.Go(func() {
				<-start
				for parameter := range 10 {
					c.getParameterValue("device", fmt.Sprint(parameter), point)
				}
			})
		}
		close(start)
		wg.Wait()
		for parameter := range 10 {
			if got := c.getParameterValue("device", fmt.Sprint(parameter), point); got != float64(timestamp) {
				t.Fatalf("parameter %d: got total %g, want %d", parameter, got, timestamp)
			}
		}
	}
}
