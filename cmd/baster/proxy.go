package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

func ProxyRequestDirector(cnf *Config, secure bool) func(*http.Request) {
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

		r.Header.Set("X-Forwarded-Host", r.Host)

		var backend string
		for _, route := range service.Route {
			if strings.HasPrefix(r.URL.Path, route.URL) {
				backend = route.K8sService
				break
			}

			// Do not take final slashes into account when checkin for prefixes if the
			// URL is the exact same path of the parent; e.g. this will match "/foo/bar/" in URL "/foo/bar"
			// but not in URL "/foo/bar2".
			if route.URL[len(route.URL)-1] == '/' && r.URL.Path == route.URL[:len(route.URL)-1] {
				backend = route.K8sService
				break
			}
		}
		if backend == "" {
			if secure {
				backend = service.K8sService
			} else {
				serveSecureRedirect(r)
				return
			}
		}

		r.URL.Scheme = "http"
		r.URL.Host = backend

		r.Header.Set("X-Baster-Backend", backend)
	}
}

func ProxyModifyResponse(resp *http.Response) error {
	start, err := time.Parse(time.RFC3339Nano, resp.Request.Header.Get("X-Baster-Start"))
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("failed parsing the start time")
		return err
	}

	log.WithFields(log.Fields{
		"backend":       resp.Request.Header.Get("X-Baster-Backend"),
		"host":          resp.Request.Host,
		"latency-ms":    int64(time.Since(start) / time.Millisecond),
		"method":        resp.Request.Method,
		"referer":       resp.Request.Header.Get("Referer"),
		"request-size":  resp.Request.ContentLength,
		"response-size": resp.ContentLength,
		"status":        resp.StatusCode,
		"time":          time.Now().Format(time.RFC3339),
		"uri":           resp.Request.Header.Get("X-Baster-Url"),
		"user-agent":    resp.Request.Header.Get("User-Agent"),
	}).Info("request")

	return nil
}

func serveStatusCode(r *http.Request, statusCode int) {
	r.URL.Scheme = "http"
	r.URL.Host = "localhost:5000"
	r.URL.Path = fmt.Sprintf("/errors/%d", statusCode)
	r.URL.RawQuery = ""

	r.Header.Set("X-Baster-Backend", "baster: errors")
}

func serveSecureRedirect(r *http.Request) {
	r.URL.Scheme = "http"
	r.URL.Host = "localhost:5000"
	r.URL.Path = "/redirects/secure"
	r.URL.RawQuery = ""

	r.Header.Set("X-Baster-Backend", "baster: redirect")
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

func ProxyRedirect(w http.ResponseWriter, r *http.Request) {
	u := new(url.URL)
	*u = *r.URL

	u.Scheme = "https"
	u.Host = r.Header.Get("X-Forwarded-Host")

	http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
}
