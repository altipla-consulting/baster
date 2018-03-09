package main

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"

	"github.com/altipla-consulting/baster/pkg/config"
	"github.com/altipla-consulting/baster/pkg/monitoring"
	"github.com/altipla-consulting/baster/pkg/proxy"
	"github.com/altipla-consulting/baster/pkg/stores"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
}

func run() error {
	if err := config.ParseSettings(); err != nil {
		return errors.Trace(err)
	}

	var manager autocert.Manager
	if !config.IsLocal() {
		cache, err := stores.NewDatastore()
		if err != nil {
			return errors.Trace(err)
		}
		log.WithFields(log.Fields{"email": config.Settings.ACME.Email}).Info("ACME account")

		var whitelist []string
		for _, domain := range config.Settings.Domains {
			whitelist = append(whitelist, domain.Hostname)
		}

		manager = autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Email:      config.Settings.ACME.Email,
			Cache:      cache,
			HostPolicy: autocert.HostWhitelist(whitelist...),
		}
	}

	if config.Settings.Redirects != "" {
		log.WithFields(log.Fields{"endpoint": config.Settings.Redirects}).Info("Configure redirects service")
	}
	if config.Settings.Monitoring.Address != {
		log.WithFields(log.Fields{
			"address": config.Settings.Monitoring.Address,
			"username": config.Settings.Monitoring.Username,
		}).Info("Configure monitoring")
	}

	hs := make(proxy.HostSwitch)
	for _, domain := range config.Settings.Domains {
		r := httprouter.New()
		r.GET("/health", proxy.HealthHandler)
		r.NotFound = http.HandlerFunc(proxy.Handler(domain))

		hs[domain.Hostname] = r
	}

	go func() {
		if config.IsLocal() {
			insecureServer := httprouter.New()
			insecureServer.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hs.ServeHTTP(w, r)
			})

			log.WithFields(log.Fields{"address": "localhost:8080"}).Info("Baster instance running")
			server := &http.Server{
				Addr:         ":8080",
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Handler:      insecureServer,
			}
			if err := server.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		} else {
			insecureServer := httprouter.New()
			insecureServer.NotFound = http.HandlerFunc(proxy.RedirectHandler)
			insecureServer.GET("/health", proxy.HealthHandler)

			server := &http.Server{
				Addr:         ":80",
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Handler:      manager.HTTPHandler(insecureServer),
			}
			if err := server.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}
	}()

	go func() {
		if !config.IsLocal() {
			secureServer := httprouter.New()
			secureServer.HandleMethodNotAllowed = false
			secureServer.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hs.ServeHTTP(w, r)
			})

			log.WithFields(log.Fields{"address": "localhost:443"}).Info("Baster instance running")
			server := &http.Server{
				Addr:         ":443",
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				Handler:      secureServer,
				TLSConfig: &tls.Config{
					GetCertificate: manager.GetCertificate,
				},
			}
			if err := server.ListenAndServeTLS("", ""); err != nil {
				log.Fatal(err)
			}
		}
	}()

	go monitoring.Sender()

	wg := new(sync.WaitGroup)
	wg.Add(1)
	wg.Wait()

	return nil
}
