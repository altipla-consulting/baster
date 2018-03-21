package monitoring

import (
	"context"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/bigquery"
	"github.com/segmentio/ksuid"

	"github.com/altipla-consulting/baster/pkg/config"
)

var bigqueryMeasurements = make(chan Measurement, 1000)

var schema = bigquery.Schema{
	&bigquery.FieldSchema{
		Name: "domain",
		Description: "Nombre del dominio que está emitiendo los logs",
		Required: true,
		Type: bigquery.StringFieldType,
	},
	&bigquery.FieldSchema{
		Name: "method",
		Description: "Método HTTP que se ha usado en la petición",
		Required: true,
		Type: bigquery.StringFieldType,
	},
	&bigquery.FieldSchema{
		Name: "status",
		Description: "Código de estado HTTP que ha dado la respuesta",
		Required: true,
		Type: bigquery.IntegerFieldType,
	},
}

type bqPoint struct {
	Domain string
	Method string
	Status int64
	Time time.Time
	Latency int64
	URL string
	Tags []string
}

func BigquerySender() {
	project, err := metadata.ProjectID()
	if err != nil {
		log.WithFields(log.Fields{"err": err.Error()}).Error("Cannot get monitoring project")
		return
	}

	log.WithFields(log.Fields{"project": project}).Info("Project detected from metadata server for monitoring")
	client, err := datastore.NewClient(context.Background(), project)
	if err != nil {
		log.WithFields(log.Fields{"err": err.Error()}).Error("Cannot initialize monitoring client")
		return
	}

	uploader := client.Dataset(config.Settings.Monitoring.BigQuery.Dataset).Table(config.Settings.Monitoring.BigQuery.Table).Uploader()

	var points []*bigquery.StructSaver

	t := time.Tick(10 * time.Second)
	for {
		select {
		case m := <-bigqueryMeasurements:
			p := &bqPoint{
				Domain: m.DomainName,
				Method: m.Method,
				Status: m.Status,
				Time: m.Time,
				Latency: m.Latency,
				URL: m.URL,
			}

			// Añade las etiquetas que el usuario defina en especial para esta ruta.
			for k, v := range m.Monitoring.Tags {
				p.Tags = append(p.Tags, fmt.Sprintf("%v=%v", k, v))
			}

			id, err := ksuid.NewRandomWithTime(m.Time)
			if err != nil {
				log.WithFields({
					"error", err.Error(),
					"monitoring": "bigquery",
				}).Error("Cannot generate insert ID for BigQuery")
				return
			}

			points = append(points, &bigQuery.StructSaver{
				Struct: p,
				Schema: schema,
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
				log.WithFields(log.Fields{
					"points": len(points),
					"monitoring": "bigquery",
				}).Warning("Discarding old points: we have too much points")
				points = points[len(points)-1000:]
			}

			// Mandamos a BigQuery los datos.
			if err := uploader.Put(ctxdeadline, points); err != nil {
				log.WithFields(log.Fields{
					"err":    err.Error(),
					"points": len(points),
					"monitoring": "bigquery",
				}).Error("Cannot write monitoring points")
			} else {
				points = nil
			}
		}
	}
}
