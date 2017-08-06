package main

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(errors.ErrorTrace(err))
	}
}

func run() error {
	server := &http.Server{
		Addr:         ":9000",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return errors.Trace(err)
	}
	return nil
}
