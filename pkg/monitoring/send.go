package monitoring

import (
	"fmt"
	"net/url"
	"time"

	"github.com/influxdata/influxdb/client"
	log "github.com/sirupsen/logrus"

	"github.com/altipla-consulting/baster/pkg/config"
)

var measurements = make(chan client.Point, 1000)

func Sender() {
	u, err := url.Parse(config.Settings.Monitoring.Address)
	if err != nil {
		log.WithFields(log.Fields{"err": err.Error()}).Error("Cannot parse monitoring address")
		return
	}

	c, err := client.NewClient(client.Config{
		URL:      *u,
		Username: config.Settings.Monitoring.Username,
		Password: config.Settings.Monitoring.Password,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		log.WithFields(log.Fields{"err": err.Error()}).Error("Cannot initialize monitoring client")
		return
	}

	var points []client.Point

	t := time.Tick(10 * time.Second)
	for {
		select {
		case p := <-measurements:
			points = append(points, p)

		case <-t:
			if len(points) > 1000 {
				log.WithFields(log.Fields{"points": len(points)}).Warning("Discarding old points: we have too much points")
				points = points[len(points)-1000:]
			}

			bp := client.BatchPoints{
				Points:   points,
				Database: "baster",
			}
			if _, err := c.Write(bp); err != nil {
				log.WithFields(log.Fields{
					"err":    err.Error(),
					"points": len(points),
				}).Error("Cannot write monitoring points")
			} else {
				points = nil
			}
		}
		time.Sleep(10 * time.Second)
	}
}

type Measurement struct {
	// Configuration data.
	Domain config.Domain
	Path   config.Path

	// Request data.
	URL    string
	Method string

	// Response data.
	Status int

	// Latency data.
	Latency int64
}

func Send(m Measurement) {
	if config.Settings.Monitoring.Address == "" {
		return
	}

	p := client.Point{
		Measurement: "latency",
		Tags: map[string]string{
			"domain": m.Domain.Name,
			"method": m.Method,
			"status": fmt.Sprintf("%d", m.Status),
		},
		Time: time.Now(),
		Fields: map[string]interface{}{
			"latency": m.Latency,
			"url":     m.URL,
		},
	}

	// Añade las etiquetas que el usuario defina en especial para esta ruta.
	for k, v := range m.Path.MonitoringTags {
		p.Tags[k] = v
	}

	measurements <- p
}
