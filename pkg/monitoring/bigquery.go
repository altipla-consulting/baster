package monitoring

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"cloud.google.com/go/compute/metadata"
	"github.com/juju/errors"
	"github.com/segmentio/ksuid"
	log "github.com/sirupsen/logrus"

	"github.com/altipla-consulting/baster/pkg/config"
)

var bigqueryMeasurements = make(chan Measurement, 1000)

var BigQuerySchema = bigquery.Schema{
	&bigquery.FieldSchema{
		Name:        "domain",
		Description: "Nombre del dominio que está emitiendo los logs",
		Required:    true,
		Type:        bigquery.StringFieldType,
	},
	&bigquery.FieldSchema{
		Name:        "method",
		Description: "Método HTTP que se ha usado en la petición",
		Required:    true,
		Type:        bigquery.StringFieldType,
	},
	&bigquery.FieldSchema{
		Name:        "status",
		Description: "Código de estado HTTP que ha dado la respuesta",
		Required:    true,
		Type:        bigquery.IntegerFieldType,
	},
	&bigquery.FieldSchema{
		Name:        "time",
		Description: "Fecha y hora en la que se ha producido la petición",
		Required:    true,
		Type:        bigquery.DateTimeFieldType,
	},
	&bigquery.FieldSchema{
		Name:        "latency",
		Description: "Latencia en milisegundos en generar la respuesta",
		Required:    true,
		Type:        bigquery.IntegerFieldType,
	},
	&bigquery.FieldSchema{
		Name:        "url",
		Description: "URL que ha pedido el usuario",
		Required:    true,
		Type:        bigquery.StringFieldType,
	},
	&bigquery.FieldSchema{
		Name:        "tags",
		Description: "Lista de etiquetas personalizadas del dominio",
		Repeated:    true,
		Type:        bigquery.StringFieldType,
	},
	&bigquery.FieldSchema{
		Name:        "referer",
		Description: "Referer de la petición. No todas las peticiones lo llevan",
		Type:        bigquery.StringFieldType,
	},
}

type bqPoint struct {
	Domain  string
	Method  string
	Status  int64
	Time    civil.DateTime
	Latency int64
	URL     string
	Tags    []string
	Referer string
}

type BigQuery struct {
	ch       chan Measurement
	uploader *bigquery.Uploader
}

func NewBigQuery(settings *config.Settings) (*BigQuery, error) {
	logger := log.WithField("monitoring", "bigquery")

	project, err := metadata.ProjectID()
	if err != nil {
		return nil, errors.Trace(err)
	}
	logger.WithField("project", project).Info("Project detected from metadata server for monitoring")

	client, err := bigquery.NewClient(context.Background(), project)
	if err != nil {
		return nil, errors.Trace(err)
	}

	bq := &BigQuery{
		uploader: client.Dataset(settings.Monitoring.BigQuery.Dataset).Table(settings.Monitoring.BigQuery.Table).Uploader(),
		ch:       make(chan Measurement, 1000),
	}

	go bq.sender()

	return bq, nil
}

func (bq *BigQuery) Send(m Measurement) {
	bq.ch <- m
}

func (bq *BigQuery) sender() {
	logger := log.WithField("monitoring", "bigquery")

	var points []*bigquery.StructSaver

	t := time.Tick(10 * time.Second)
	for {
		select {
		case m := <-bq.ch:
			p := &bqPoint{
				Domain:  m.DomainName,
				Method:  m.Method,
				Status:  int64(m.Status),
				Time:    civil.DateTimeOf(m.Time),
				Latency: m.Latency,
				URL:     m.URL,
				Referer: m.Referer,
			}

			// Añade las etiquetas que el usuario defina en especial para esta ruta.
			for k, v := range m.Monitoring.Tags {
				p.Tags = append(p.Tags, fmt.Sprintf("%v=%v", k, v))
			}

			id, err := ksuid.NewRandomWithTime(m.Time)
			if err != nil {
				logger.WithField("err", err.Error()).Fatal("Cannot generate ksuid ID")
				return
			}

			points = append(points, &bigquery.StructSaver{
				Struct:   p,
				Schema:   BigQuerySchema,
				InsertID: id.String(),
			})

		case <-t:
			// No hay peticiones en los últimos 10 segundos.
			if len(points) == 0 {
				continue
			}

			// Se han ido acumulando demasiados puntos por problemas de conexión en
			// antiguos envíos; empezamos a descartar puntos antiguos.
			if len(points) > 1000 {
				logger.WithField("points", len(points)).Warning("Discarding old points: we have too much points")
				points = points[len(points)-1000:]
			}

			// Mandamos a BigQuery los datos.
			ctxdeadline, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			if err := bq.uploader.Put(ctxdeadline, points); err != nil {
				logger.WithFields(log.Fields{"err": err.Error(), "points": len(points)}).Error("Cannot write monitoring points")
				cancel()
			} else {
				points = nil
				cancel()
			}
		}
	}
}
