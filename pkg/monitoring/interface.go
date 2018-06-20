package monitoring

import (
	"time"

	"github.com/altipla-consulting/baster/pkg/config"
)

type Interface interface {
	Send(m Measurement)
}

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

func Send(instances []Interface, m Measurement) {
	if m.Monitoring.Name != "" {
		m.DomainName = m.Monitoring.Name
	}
	m.Time = time.Now()

	for _, instance := range instances {
		instance.Send(m)
	}
}
