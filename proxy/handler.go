package proxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"baster/config"
)

func NewInsecureHandler(cnf *config.Config) http.Handler {
	return newHandler(cnf, false)
}

func NewSecureHandler(cnf *config.Config) http.Handler {
	return newHandler(cnf, true)
}

func newHandler(cnf *config.Config, secure bool) http.Handler {
	servicesByHostname := map[string]config.Service{}
	for _, service := range cnf.Service {
		servicesByHostname[service.Hostname] = service
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqURL := r.URL

		r.Header.Set("X-Forwarded-Host", r.Host)

		if reqURL.Path == "/health" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, "baster is ok")
			return
		}

		service, ok := servicesByHostname[r.Host]
		if !ok {
			log.WithFields(log.Fields{"hostname": r.Host}).Warning("service not found")
			http.Error(w, "baster: service not found", http.StatusNotFound)
			return
		}

		u := new(url.URL)
		*u = *reqURL

		var backend string
		for _, route := range service.Route {
			if strings.HasPrefix(reqURL.Path, route.URL) {
				backend = route.K8sService
				break
			}

			// Do not take final slashes into account when checkin for prefixes if the
			// URL is the exact same path of the parent; e.g. this will match "/foo/bar/" in URL "/foo/bar"
			// but not in URL "/foo/bar2".
			if route.URL[len(route.URL)-1] == '/' && reqURL.Path == route.URL[:len(route.URL)-1] {
				backend = route.K8sService
				break
			}
		}
		if backend == "" {
			if secure {
				backend = service.K8sService
			} else {
				u.Scheme = "https"
				u.Host = r.Host

				http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
				return
			}
		}

		u.Scheme = "http"
		u.Host = backend
		r.URL = u

		var response *http.Response
		proxy := &httputil.ReverseProxy{
			Director:      func(r *http.Request) {},
			FlushInterval: 60 * time.Second,
			Transport:     new(Transport),
			ModifyResponse: func(resp *http.Response) error {
				response = resp
				return nil
			},
		}
		proxy.ServeHTTP(w, r)

		log.WithFields(log.Fields{
			"backend":       backend,
			"host":          r.Host,
			"latency-ms":    int64(time.Since(start) / time.Millisecond),
			"method":        r.Method,
			"referer":       r.Header.Get("Referer"),
			"request-size":  r.ContentLength,
			"response-size": response.ContentLength,
			"secure":        secure,
			"status":        response.StatusCode,
			"time":          time.Now().Format(time.RFC3339),
			"uri":           reqURL.String(),
			"user-agent":    r.Header.Get("User-Agent"),
		}).Info("request")
	})
}

type Transport struct{}

func (transport *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("transport error")
		resp = &http.Response{
			Status:     fmt.Sprintf("%d %s", http.StatusBadGateway, http.StatusText(http.StatusBadGateway)),
			StatusCode: http.StatusBadGateway,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Body:       ioutil.NopCloser(bytes.NewBuffer(nil)),
			Request:    r,
		}
	}

	return resp, nil
}
