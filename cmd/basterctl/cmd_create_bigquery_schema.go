package main

import (
  "cloud.google.com/go/bigquery"
  "github.com/spf13/cobra"
  "golang.org/x/net/context"
  "github.com/juju/errors"

  "github.com/altipla-consulting/baster/pkg/monitoring"
)

var project, dataset string

func init() {
  CmdRoot.AddCommand(CmdCreateBigQuerySchema)
  CmdCreateBigQuerySchema.PersistentFlags().StringVarP(&project, "project", "", "", "Nombre del proyecto de Google Cloud")
  CmdCreateBigQuerySchema.PersistentFlags().StringVarP(&dataset, "dataset", "", "", "Nombre del dataset creado manualmente")
}

var CmdCreateBigQuerySchema = &cobra.Command{
  Use:   "create-bigquery-schema",
  Short: "Crea la tabla que albergará registros de monitorización de Baster en BigQuery.",
  RunE: func(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    if project == "" {
      return errors.NotValidf("--project argument is required")
    }
    if dataset == "" {
      return errors.NotValidf("--dataset argument is required")
    }

    client, err := bigquery.NewClient(ctx, project)
    if err != nil {
      return errors.Trace(err)
    }

    table := client.Dataset(dataset).Table("baster")
    metadata := &bigquery.TableMetadata{
      Description: "Registro de peticiones de baster",
      Schema: monitoring.BigQuerySchema,
      TimePartitioning: new(bigquery.TimePartitioning),
    }
    if err := table.Create(ctx, metadata); err != nil {
      return errors.Trace(err)
    }

    return nil
  },
}
