package monitoring

import (
	"github.com/altipla-consulting/baster/pkg/config"
)

type Measurement struct {
	// Configuration data.
	DomainName string
	Monitoring config.PathMonitoring

	// Request data.
	URL    string
	Method string

	// Response data.
	Status int

	// Latency data.
	Latency int64
}

func Send(m Measurement) {
	if m.Monitoring.Name != "" {
		m.DomainName = m.Monitoring.Name
	}

	if config.Settings.Monitoring.Address != "" {
		influxDBMeasurements <- m
	}
}
