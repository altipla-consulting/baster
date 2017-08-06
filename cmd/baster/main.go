package main

import (
	"crypto/tls"
	"flag"
	"net/http"
	"net/http/httputil"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
}

func run() error {
	flag.Parse()

	if IsDebug() {
		log.SetFormatter(&log.TextFormatter{
			ForceColors:   true,
			FullTimestamp: true,
		})
	}

	cnf, err := LoadConfig()
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
	if IsDebug() {
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
			Handler: &httputil.ReverseProxy{
				Director:       ProxyRequestDirector(cnf, false),
				FlushInterval:  60 * time.Second,
				ModifyResponse: ProxyModifyResponse,
				Transport:      NewProxyTransport(),
			},
		}
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(errors.ErrorStack(err))
		}
	}()

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/errors/404", ProxyError(http.StatusNotFound))
		mux.HandleFunc("/errors/502", ProxyError(http.StatusBadGateway))
		mux.HandleFunc("/redirects/secure", ProxyRedirect)
		mux.HandleFunc("/health", ProxyHealth)

		log.WithFields(log.Fields{"address": "localhost:5000"}).Info("run errors server")
		server := &http.Server{
			Addr:         ":5000",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			Handler:      mux,
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
		Handler: &httputil.ReverseProxy{
			Director:       ProxyRequestDirector(cnf, true),
			FlushInterval:  60 * time.Second,
			ModifyResponse: ProxyModifyResponse,
			Transport:      NewProxyTransport(),
		},
		TLSConfig: &tls.Config{
			GetCertificate: manager.GetCertificate,
		},
	}
	if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return errors.Trace(err)
	}

	return nil
}
