package main

import (
	"flag"
	"net/http"
	"net/http/httputil"
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

	proxy := &httputil.ReverseProxy{
		Director:       ProxyRequestDirector(cnf),
		FlushInterval:  60 * time.Second,
		ModifyResponse: ProxyModifyResponse,
		Transport:      NewProxyTransport(),
	}

	go func() {
		log.WithFields(log.Fields{"address": "localhost:9080"}).Info("run insecure server")
		server := &http.Server{
			Addr:         ":9080",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			Handler:      proxy,
		}
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(errors.ErrorStack(err))
		}
	}()

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/errors/404", ProxyError(http.StatusNotFound))
		mux.HandleFunc("/errors/502", ProxyError(http.StatusBadGateway))

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

	log.WithFields(log.Fields{"address": "localhost:9443"}).Info("run secure server")
	server := &http.Server{
		Addr:         ":9443",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      proxy,
	}
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return errors.Trace(err)
	}

	return nil
}
