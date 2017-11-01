package main

import (
	"net/http"
	"time"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"

	"baster/pkg/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
}

func run() error {
	if config.IsDebug() {
		log.SetFormatter(&log.TextFormatter{
			ForceColors:   true,
			FullTimestamp: true,
		})
	}

	go func() {
		log.WithFields(log.Fields{"address": "localhost:9000"}).Info("run local verification server")
		server := &http.Server{
			Addr:         ":9000",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Handler:      verificationHandler,
		}
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(errors.ErrorStack(err))
		}
	}()

	for {
		cnf, changed, err := detectConfigChange()
		if err != nil {
			return errors.Trace(err)
		}

		if changed {
			log.WithFields(log.Fields{"version": cnf.Version}).Info("new configmap version loaded")

			log.Info("first reload of nginx")
			if err := reloadNginxConfig(cnf); err != nil {
				return errors.Trace(err)
			}

			log.Info("renew certificates")
			if err := verifyCertificates(cnf); err != nil {
				return errors.Trace(err)
			}

			log.Info("final reload of nginx")
			if err := reloadNginxConfig(cnf); err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}
