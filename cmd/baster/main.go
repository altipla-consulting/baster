package main

import (
	"flag"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
}

func run() error {
	flag.Parse()

	log.SetFormatter(&log.TextFormatter{
		ForceColors: true,
	})

	cnf, err := LoadConfig()
	if err != nil {
		return errors.Trace(err)
	}

	_ = cnf

	log.WithFields(log.Fields{"address": "localhost:9443"}).Info("run secure server")
	server := &http.Server{
		Addr:         ":9443",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return errors.Trace(err)
	}
	return nil
}
