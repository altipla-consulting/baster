package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/altipla-consulting/collections"
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"

	"github.com/altipla-consulting/baster/pkg/config"
	"github.com/altipla-consulting/baster/pkg/monitoring"
)

func proxyHandler(settings *config.Settings, domain *config.Domain, monitors []monitoring.Interface) http.HandlerFunc {
	defaultPath := &config.Path{
		Service: domain.Service,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Activa las cabeceras CORS en peticiones que cruzan dominios si coincide
		// con un origen autorizado en la configuración.
		origin := r.Header.Get("Origin")
		if collections.HasString(domain.CORS.Origins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Aplica redirecciones de dominio si están configuradas
		if domain.Redirect != "" {
			u := r.URL
			u.Scheme = "https"
			u.Host = domain.Redirect
			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
			return
		}

		// Aplica autenticación externa si está configurada.
		if settings.Auth.Endpoint != "" {
			if keep, err := queryAuth(settings, w, r); err != nil {
				log.WithFields(log.Fields{"error": err.Error(), "stack": errors.ErrorStack(err)}).Error("Auth failed")
				http.Error(w, "auth failed", http.StatusInternalServerError)
				return
			} else if !keep {
				return
			}
		}

		// Aplica el servicio de redirecciones si lo hemos configurado.
		source := fmt.Sprintf("https://%s%s", r.Host, r.URL.String())
		if settings.Redirects.Apply != "" {
			dest, err := queryRedirect(settings, source)
			if err != nil {
				log.WithFields(log.Fields{"error": err.Error(), "stack": errors.ErrorStack(err)}).Error("Redirects failed")
				http.Error(w, "redirects failed", http.StatusInternalServerError)
				return
			}

			if dest != source {
				http.Redirect(w, r, dest, http.StatusMovedPermanently)
				return
			}
		}

		var path *config.Path
		for _, p := range domain.Paths {
			if strings.HasPrefix(r.URL.Path, p.Match) {
				path = p
				break
			}
		}
		if path == nil {
			path = defaultPath
		}

		// Guarda algunos valores para evitar que se sobreescriban luego y podamos emitirlos.
		host := r.Host
		reqURL := new(url.URL)
		*reqURL = *r.URL

		// Reescribe la URL de destino.
		r.URL.Scheme = "http"
		r.URL.Host = path.Service

		// Reescribe la cabecera Host.
		r.Host = path.Service
		if domain.VirtualHostname != "" {
			r.Host = domain.VirtualHostname
		}
		r.Header.Set("X-Forwarded-Host", host)

		// Añade las cabeceras de salto.
		for key, value := range domain.HopHeaders {
			r.Header.Set(key, value)
		}

		// Ejecuta el proxy de la petición.
		var resp *http.Response
		proxy := &httputil.ReverseProxy{
			Director:      func(r *http.Request) {},
			FlushInterval: 10 * time.Second,
			Transport:     new(transport),
			ModifyResponse: func(response *http.Response) error {
				resp = response
				return nil
			},
		}
		proxy.ServeHTTP(w, r)

		if shouldLogRequest(r) {
			// Intenta averiguar la longitud del contenido que estamos sirviendo.
			length := resp.ContentLength
			if length == -1 {
				length, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
			}

			latency := int64(time.Since(start) / time.Millisecond)
			monitoring.Send(monitors, monitoring.Measurement{
				DomainName: domain.Name,
				Monitoring: path.Monitoring,
				URL:        source,
				Method:     r.Method,
				Referer:    r.Referer(),
				Status:     resp.StatusCode,
				Latency:    latency,
				Time:       time.Now(),
			})

			// Logging de la petición que hemos recibido.
			log.WithFields(log.Fields{
				"domain":        domain.Name,
				"host":          host,
				"service":       r.URL.Host,
				"method":        r.Method,
				"referer":       r.Header.Get("Referer"),
				"request-size":  r.ContentLength,
				"uri":           reqURL.String(),
				"user-agent":    r.Header.Get("User-Agent"),
				"authorization": r.Header.Get("Authorization"),
				"secure":        r.TLS != nil,
				"latency-ms":    latency,
				"resp-size":     length,
				"status":        resp.StatusCode,
			}).Info("request")
		}
	}
}

type transport struct{}

func (transport *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Transport error")

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

type redirectRequest struct {
	Source string `json:"source"`
}

type redirectReply struct {
	Destination string `json:"destination"`
}

func queryRedirect(settings *config.Settings, url string) (string, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&redirectRequest{url}); err != nil {
		return "", errors.Trace(err)
	}

	req, _ := http.NewRequest("POST", settings.Redirects.Apply, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected status code: %v", resp.Status)
	}

	reply := new(redirectReply)
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return "", errors.Trace(err)
	}

	return reply.Destination, nil
}

func queryAuth(settings *config.Settings, w http.ResponseWriter, r *http.Request) (bool, error) {
	authEndpoint, err := url.Parse(settings.Auth.Endpoint)
	if err != nil {
		return false, errors.Trace(err)
	}

	qs := authEndpoint.Query()
	qs.Set("hostname", r.Host)
	qs.Set("url", r.URL.String())
	qs.Set("authorization", r.Header.Get("Authorization"))
	authEndpoint.RawQuery = qs.Encode()

	req, err := http.NewRequest("GET", authEndpoint.String(), nil)
	if err != nil {
		return false, errors.Trace(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, errors.Trace(err)
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, errors.Trace(err)
	}

	switch resp.StatusCode {
	case http.StatusForbidden:
		http.Error(w, string(content), http.StatusForbidden)
		return false, nil

	case http.StatusOK:
		return true, nil

	case http.StatusFound:
		http.Redirect(w, r, resp.Header.Get("Location"), http.StatusFound)
		return false, nil

	default:
		return false, errors.Errorf("unexpected auth status: %v", resp.Status)
	}
}

func shouldLogRequest(r *http.Request) bool {
	return r.UserAgent() != "GoogleStackdriverMonitoring-UptimeChecks(https://cloud.google.com/monitoring)"
}
