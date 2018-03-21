package monitoring

import (
	"fmt"
	"net/url"
	"time"

	"github.com/influxdata/influxdb/client"
	log "github.com/sirupsen/logrus"

	"github.com/altipla-consulting/baster/pkg/config"
)

var influxDBMeasurements = make(chan Measurement, 1000)

func InfluxDBSender() {
	u, err := url.Parse(config.Settings.Monitoring.InfluxDB.Address)
	if err != nil {
		log.WithFields(log.Fields{"err": err.Error()}).Error("Cannot parse monitoring address")
		return
	}

	c, err := client.NewClient(client.Config{
		URL:      *u,
		Username: config.Settings.Monitoring.InfluxDB.Username,
		Password: config.Settings.Monitoring.InfluxDB.Password,
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
		case m := <-influxDBMeasurements:
			p := client.Point{
				Measurement: "latency",
				Tags: map[string]string{
					"domain": m.DomainName,
					"method": m.Method,
					"status": fmt.Sprintf("%d", m.Status),
				},
				Time: m.Time,
				Fields: map[string]interface{}{
					"latency": m.Latency,
					"url":     m.URL,
					"value":   1,
				},
			}

			// Añade las etiquetas que el usuario defina en especial para esta ruta.
			for k, v := range m.Monitoring.Tags {
				p.Tags[k] = v
			}

			points = append(points, p)

		case <-t:
			// No hay peticiones en los últimos 10 segundos.
			if len(points) == 0 {
				continue
			}

			// Se han ido acumulando demasiados puntos por problemas de conexión en
			// antiguos envíos; empezamos a descartar puntos antiguos.
			if len(points) > 1000 {
				log.WithFields(log.Fields{
					"points": len(points),
					"monitoring": "influxdb",
				}).Warning("Discarding old points: we have too much points")
				points = points[len(points)-1000:]
			}

			// Mandamos a InfluxDB los datos.
			bp := client.BatchPoints{
				Points:   points,
				Database: "baster",
			}
			if _, err := c.Write(bp); err != nil {
				log.WithFields(log.Fields{
					"err":    err.Error(),
					"points": len(points),
					"monitoring": "influxdb",
				}).Error("Cannot write monitoring points")
			} else {
				points = nil
			}
		}
	}
}
