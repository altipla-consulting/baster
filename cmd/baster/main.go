package main

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"

	"baster/pkg/config"
	"baster/pkg/proxy"
	"baster/pkg/store"
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

	updates := make(chan *config.Config, 1)
	go config.AutoUpdate(updates)

	// We need the config for the ACME email
	cnf := <-updates

	p := proxy.New(updates)
	handler := p.Handler()

	// Restore the config in the queue again to receive it in the proxy controller.
	updates <- cnf

	cache, err := store.NewDatastore()
	if err != nil {
		return errors.Trace(err)
	}
	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Email:      cnf.ACME.Email,
		Cache:      cache,
		HostPolicy: p.HostPolicy(),
	}

	go func() {
		log.WithFields(log.Fields{"address": "localhost:80"}).Info("run insecure server")
		server := &http.Server{
			Addr:         ":80",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Handler:      handler,
		}
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(errors.ErrorStack(err))
		}
	}()

	log.WithFields(log.Fields{"address": "localhost:443", "acme-email": cnf.ACME.Email}).Info("run secure server")
	server := &http.Server{
		Addr:         ":443",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      handler,
		TLSConfig: &tls.Config{
			GetCertificate: manager.GetCertificate,
		},
	}
	if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return errors.Trace(err)
	}

	return nil
}
