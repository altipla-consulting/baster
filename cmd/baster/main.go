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

	var cnf *config.Config
	for {
		var err error
		cnf, err = config.Load()
		if err != nil {
			return errors.Trace(err)
		}

		if cnf == nil {
			log.Errorf("no configmap with the name baster found, retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}

		if err := cnf.IsValid(); err != nil {
			log.WithFields(log.Fields{"error": err}).Errorf("configuration is not valid, retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}

		break
	}

	ctrl := cnf.NewController()

	cache, err := store.NewDatastore()
	if err != nil {
		return errors.Trace(err)
	}
	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Email:      cnf.ACME.Email,
		Cache:      cache,
		HostPolicy: ctrl.HostPolicy(),
	}

	go ctrl.AutoUpdate()

	go func() {
		log.WithFields(log.Fields{"address": "localhost:80"}).Info("run insecure server")
		server := &http.Server{
			Addr:         ":80",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Handler:      proxy.NewInsecureHandler(ctrl),
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
		Handler:      proxy.NewSecureHandler(ctrl),
		TLSConfig: &tls.Config{
			GetCertificate: manager.GetCertificate,
		},
	}
	if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return errors.Trace(err)
	}

	return nil
}
