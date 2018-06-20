package main

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"

	"github.com/altipla-consulting/baster/pkg/config"
	"github.com/altipla-consulting/baster/pkg/proxy"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
}

func run() error {
	watcher, err := proxy.NewServerWatcher()
	if err != nil {
		return errors.Trace(err)
	}

	if config.IsLocal() {
		log.WithFields(log.Fields{"address": "localhost:8080"}).Info("Baster local instance running")
		server := &http.Server{
			Addr:         ":8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Handler:      http.HandlerFunc(watcher.SecureServeHTTP),
		}
		if err := server.ListenAndServe(); err != nil {
			return errors.Trace(err)
		}

		return nil
	}

	go func() {
		log.WithField("address", "localhost:80").Info("Baster insecure instance running")
		server := &http.Server{
			Addr:         ":80",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Handler:      http.HandlerFunc(watcher.InsecureServeHTTP),
		}
		if err := server.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	log.WithField("address", "localhost:443").Info("Baster secure instance running")
	server := &http.Server{
		Addr:         ":443",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		Handler:      http.HandlerFunc(watcher.SecureServeHTTP),
		TLSConfig: &tls.Config{
			GetCertificate: watcher.GetCertificate,
		},
	}
	if err := server.ListenAndServeTLS("", ""); err != nil {
		return errors.Trace(err)
	}

	return nil
}
