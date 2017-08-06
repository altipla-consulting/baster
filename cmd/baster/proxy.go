package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

func ProxyRequestDirector(cnf *Config) func(*http.Request) {
	servicesByHostname := map[string]Service{}
	for _, service := range cnf.Service {
		servicesByHostname[service.Hostname] = service
	}

	return func(r *http.Request) {
		r.Header.Set("X-Baster-Start", time.Now().Format(time.RFC3339Nano))
		r.Header.Set("X-Baster-Url", r.URL.String())

		service, ok := servicesByHostname[r.Host]
		if !ok {
			log.WithFields(log.Fields{"hostname": r.Host}).Warning("host not found")
			serveStatusCode(r, http.StatusNotFound)
			return
		}

		backend := service.K8sService
		for _, route := range service.Route {
			if strings.HasPrefix(r.URL.Path, route.URL) {
				backend = route.K8sService
				break
			}

			if route.URL[len(route.URL)-1] == '/' && r.URL.Path == route.URL[:len(route.URL)-1] {
				backend = route.K8sService
				break
			}
		}

		r.URL.Scheme = "http"
		r.URL.Host = backend
	}
}

func ProxyModifyResponse(resp *http.Response) error {
	start, err := time.Parse(time.RFC3339Nano, resp.Request.Header.Get("X-Baster-Start"))
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("failed parsing the start time")
		return err
	}

	log.WithFields(log.Fields{
		"host":          resp.Request.Host,
		"method":        resp.Request.Method,
		"referer":       resp.Request.Header.Get("Referer"),
		"request-size":  resp.Request.ContentLength,
		"response-size": resp.ContentLength,
		"status":        resp.StatusCode,
		"time":          time.Now().Format(time.RFC3339),
		"uri":           resp.Request.Header.Get("X-Baster-Url"),
		"user-agent":    resp.Request.Header.Get("User-Agent"),
		"latency-ms":    int64(time.Since(start) / time.Millisecond),
	}).Info("request")

	return nil
}

func serveStatusCode(r *http.Request, statusCode int) {
	r.URL.Scheme = "http"
	r.URL.Host = "localhost:5000"
	r.URL.Path = fmt.Sprintf("/errors/%d", statusCode)
	r.URL.RawQuery = ""
}

func ProxyError(statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)), statusCode)
	}
}

type ProxyTransport struct{}

func NewProxyTransport() http.RoundTripper {
	return new(ProxyTransport)
}

func (transport *ProxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("transport error")
		serveStatusCode(r, http.StatusBadGateway)

		return http.DefaultTransport.RoundTrip(r)
	}

	return resp, nil
}
