package main

import (
	"crypto/tls"
	"flag"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"baster/config"
	"baster/proxy"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
}

func run() error {
	flag.Parse()

	if config.IsDebug() {
		log.SetFormatter(&log.TextFormatter{
			ForceColors:   true,
			FullTimestamp: true,
		})
	}

	cnf, err := config.Load()
	if err != nil {
		return errors.Trace(err)
	}

	hosts := []string{}
	for _, service := range cnf.Service {
		hosts = append(hosts, service.Hostname)
	}
	cache, err := NewDatastoreCache(cnf)
	if err != nil {
		return errors.Trace(err)
	}
	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(hosts...),
		Email:      cnf.ACMEEmail,
		Cache:      cache,
	}
	if config.IsDebug() {
		manager.Client = &acme.Client{
			DirectoryURL: "https://acme-staging.api.letsencrypt.org/directory",
		}
	}

	go func() {
		log.WithFields(log.Fields{"address": "localhost:9080"}).Info("run insecure server")
		server := &http.Server{
			Addr:         ":9080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Handler:      proxy.NewInsecureHandler(cnf),
		}
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(errors.ErrorStack(err))
		}
	}()

	log.WithFields(log.Fields{
		"address":    "localhost:9443",
		"acme-email": cnf.ACMEEmail,
		"hosts":      hosts,
	}).Info("run secure server")

	server := &http.Server{
		Addr:         ":9443",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      proxy.NewSecureHandler(cnf),
		TLSConfig: &tls.Config{
			GetCertificate: manager.GetCertificate,
		},
	}
	if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return errors.Trace(err)
	}

	return nil
}
