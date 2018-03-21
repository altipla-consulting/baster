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

func Handler(domain *config.Domain) http.HandlerFunc {
	log.WithFields(log.Fields{
		"hostname":         domain.Hostname,
		"service":          domain.Service,
		"virtual-hostname": domain.VirtualHostname,
		"cors-origins":     domain.CORS.Origins,
		"hop-headers":      domain.HopHeaders,
	}).Info("Domain configured")

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Aplica el servicio de redirecciones si lo hemos configurado.
		source := fmt.Sprintf("https://%s%s", r.Host, r.URL.String())
		if config.Settings.Redirects.Apply != "" {
			dest, err := queryRedirect(source)
			if err != nil {
				http.Error(w, "Redirects not working", http.StatusInternalServerError)
				return
			}

			if dest != source {
				http.Redirect(w, r, dest, http.StatusMovedPermanently)
				return
			}
		}

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

		var path *config.Path
		for _, p := range domain.Paths {
			if strings.HasPrefix(r.URL.Path, p.Match) {
				path = p
				break
			}
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

		// Intenta averiguar la longitud del contenido que estamos sirviendo.
		length := resp.ContentLength
		if length == -1 {
			length, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
		}

		latency := int64(time.Since(start) / time.Millisecond)
		monitoring.Send(monitoring.Measurement{
			DomainName: domain.Name,
			Monitoring: path.Monitoring,
			URL:        source,
			Method:     r.Method,
			Status:     resp.StatusCode,
			Latency:    latency,
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

func queryRedirect(url string) (string, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&redirectRequest{url}); err != nil {
		return "", errors.Trace(err)
	}

	req, _ := http.NewRequest("POST", config.Settings.Redirects.Apply, &buf)
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
