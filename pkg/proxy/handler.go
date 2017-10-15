package proxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"baster/pkg/config"
)

func NewInsecureHandler(ctrl *config.Controller) http.Handler {
	return newHandler(ctrl, false)
}

func NewSecureHandler(ctrl *config.Controller) http.Handler {
	return newHandler(ctrl, true)
}

type handleConfig struct {
	Endpoint      string
	AllowInsecure bool
}

func newHandler(ctrl *config.Controller, secure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqURL := r.URL

		r.Header.Set("X-Forwarded-Host", r.Host)

		if reqURL.Path == "/health" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, "baster is ok")
			return
		}

		name, service := ctrl.FindService(r.Host)
		if service == nil {
			log.WithFields(log.Fields{"hostname": r.Host}).Warning("service not found")
			http.Error(w, "baster: service not found", http.StatusNotFound)
			return
		}

		u := new(url.URL)
		*u = *reqURL

		match := u.Path
		if strings.HasSuffix(match, "/") {
			match = match[:len(match)-1]
		}

		handleCnf := &handleConfig{
			Endpoint:      service.Endpoint,
			AllowInsecure: service.AllowInsecure,
		}
		for _, route := range service.Routes {
			compare := route.URL
			if strings.HasSuffix(compare, "/") {
				compare = compare[:len(compare)-1]
			}

			if route.ExactMatch {
				if match != compare {
					continue
				}
			} else {
				if !strings.HasPrefix(match, compare) {
					continue
				}
			}

			if route.AllowInsecure {
				handleCnf.AllowInsecure = true
			}
			if route.Endpoint != "" {
				handleCnf.Endpoint = route.Endpoint
			}
		}

		if !handleCnf.AllowInsecure && !secure {
			u.Scheme = "https"
			u.Host = r.Host
			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)

			log.WithFields(log.Fields{
				"service":       name,
				"endpoint":      handleCnf.Endpoint,
				"host":          r.Host,
				"latency-ms":    int64(time.Since(start) / time.Millisecond),
				"method":        r.Method,
				"referer":       r.Header.Get("Referer"),
				"request-size":  r.ContentLength,
				"secure":        secure,
				"status":        http.StatusMovedPermanently,
				"time":          time.Now().Format(time.RFC3339),
				"uri":           reqURL.String(),
				"user-agent":    r.Header.Get("User-Agent"),
				"authorization": r.Header.Get("Authorization"),
			}).Info("redirected insecure request")

			return
		}

		if r.Method == "OPTIONS" && service.CORSEnabled {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)

			log.WithFields(log.Fields{
				"service":       name,
				"endpoint":      handleCnf.Endpoint,
				"host":          r.Host,
				"latency-ms":    int64(time.Since(start) / time.Millisecond),
				"method":        r.Method,
				"referer":       r.Header.Get("Referer"),
				"request-size":  r.ContentLength,
				"secure":        secure,
				"status":        http.StatusMovedPermanently,
				"time":          time.Now().Format(time.RFC3339),
				"uri":           reqURL.String(),
				"user-agent":    r.Header.Get("User-Agent"),
				"authorization": r.Header.Get("Authorization"),
			}).Info("cors request")

			return
		}

		u.Scheme = "http"
		u.Host = handleCnf.Endpoint
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
			"service":       name,
			"endpoint":      handleCnf.Endpoint,
			"host":          r.Host,
			"latency-ms":    int64(time.Since(start) / time.Millisecond),
			"method":        r.Method,
			"referer":       r.Header.Get("Referer"),
			"request-size":  r.ContentLength,
			"response-size": length,
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
