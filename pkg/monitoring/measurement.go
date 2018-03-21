package monitoring

import (
	"time"

	"github.com/altipla-consulting/baster/pkg/config"
)

type Measurement struct {
	// Configuration data.
	DomainName string
	Monitoring config.PathMonitoring

	// Request data.
	URL     string
	Method  string
	Referer string

	// Response data.
	Status int

	// Latency data.
	Latency int64

	// Autofilled by the send procedure
	Time time.Time
}

func Send(m Measurement) {
	if m.Monitoring.Name != "" {
		m.DomainName = m.Monitoring.Name
	}
	m.Time = time.Now()

	if config.Settings.Monitoring.InfluxDB.Address != "" {
		influxDBMeasurements <- m
	}
	if config.Settings.Monitoring.BigQuery.Dataset != "" {
		bigqueryMeasurements <- m
	}
}
