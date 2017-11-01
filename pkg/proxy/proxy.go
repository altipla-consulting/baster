package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"

	"baster/pkg/config"
)

type Proxy struct {
	*sync.RWMutex

	services map[string]*Service
}

type Service struct {
	Name     string
	Endpoint string
	Routes   []*Route
}

func (service *Service) FindRoute(search string) *Route {
	for _, route := range service.Routes {
		if route.Matches(search) {
			return route
		}
	}

	return nil
}

type Route struct {
	URL           string
	AllowInsecure bool
	Endpoint      string
	ExactMatch    bool
	CORSEnabled   bool
}

func (route *Route) Matches(search string) bool {
	if route.ExactMatch {
		return search == route.URL
	}

	return strings.HasPrefix(search, route.URL)
}

func New(configUpdates chan *config.Config) *Proxy {
	ctrl := &Proxy{
		RWMutex: new(sync.RWMutex),
	}

	go func() {
		for {
			cnf := <-configUpdates

			ctrl.Lock()
			defer ctrl.Unlock()

			ctrl.services = map[string]*Service{}
			for name, service := range cnf.Services {
				// Service configured routes.
				routes := []*Route{}
				for _, route := range routes {
					endpoint := route.Endpoint
					if endpoint == "" {
						endpoint = service.Endpoint
					}

					routes = append(routes, &Route{
						URL:           route.URL,
						AllowInsecure: route.AllowInsecure || service.AllowInsecure,
						Endpoint:      endpoint,
						ExactMatch:    route.ExactMatch,
						CORSEnabled:   service.CORSEnabled,
					})
				}

				// Default fallback route for all services.
				routes = append(routes, &Route{
					URL:           "/",
					AllowInsecure: service.AllowInsecure,
					Endpoint:      service.Endpoint,
					CORSEnabled:   service.CORSEnabled,
				})

				// Register the service.
				ctrl.services[service.Hostname] = &Service{
					Name:     name,
					Endpoint: service.Endpoint,
					Routes:   routes,
				}
			}
		}
	}()

	return ctrl
}

func (ctrl *Proxy) HostPolicy() autocert.HostPolicy {
	return func(ctx context.Context, domain string) error {
		ctrl.RLock()
		defer ctrl.RUnlock()

		if _, ok := ctrl.services[domain]; !ok {
			return errors.Errorf("unknown service hostname: %s", domain)
		}

		return nil
	}
}

func (ctrl *Proxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		reqURL := new(url.URL)
		*reqURL = *r.URL

		r.Header.Set("X-Forwarded-Host", r.Host)

		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, "baster is ok")
			return
		}

		ctrl.RLock()
		service := ctrl.services[r.Host]
		ctrl.RUnlock()

		if service == nil {
			log.WithFields(log.Fields{"hostname": r.Host}).Error("service not found")
			http.Error(w, "baster: service not found", http.StatusNotFound)
			return
		}

		u := new(url.URL)
		*u = *reqURL

		match := u.Path
		if strings.HasSuffix(match, "/") {
			match = match[:len(match)-1]
		}
		route := service.FindRoute(match)

		reqLogger := log.WithFields(log.Fields{
			"service":       service.Name,
			"host":          r.Host,
			"endpoint":      route.Endpoint,
			"method":        r.Method,
			"referer":       r.Header.Get("Referer"),
			"request-size":  r.ContentLength,
			"uri":           reqURL.String(),
			"user-agent":    r.Header.Get("User-Agent"),
			"authorization": r.Header.Get("Authorization"),
			"secure":        r.TLS != nil,
		})

		// HTTP -> HTTPS redirection.
		if !route.AllowInsecure && r.TLS == nil {
			u.Scheme = "https"
			u.Host = r.Host
			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)

			reqLogger.WithFields(log.Fields{
				"latency-ms": int64(time.Since(start) / time.Millisecond),
				"status":     http.StatusMovedPermanently,
			}).Info("redirect insecure request")

			return
		}

		if r.Method == "OPTIONS" && route.CORSEnabled {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)

			log.WithFields(log.Fields{
				"latency-ms": int64(time.Since(start) / time.Millisecond),
				"status":     http.StatusOK,
			}).Info("cors authorization")

			return
		}

		u.Scheme = "http"
		u.Host = route.Endpoint
		r.URL = u

		var response *http.Response
		proxy := &httputil.ReverseProxy{
			Director:      func(r *http.Request) {},
			FlushInterval: 10 * time.Second,
			Transport:     new(Transport),
			ModifyResponse: func(resp *http.Response) error {
				response = resp
				return nil
			},
		}
		proxy.ServeHTTP(w, r)

		length := response.ContentLength
		if length == -1 {
			length, _ = strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
		}

		log.WithFields(log.Fields{
			"latency-ms":    int64(time.Since(start) / time.Millisecond),
			"response-size": length,
			"status":        response.StatusCode,
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
