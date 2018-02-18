package proxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/altipla-consulting/collections"
	log "github.com/sirupsen/logrus"

	"github.com/altipla-consulting/baster/pkg/config"
)

var assetsExts = []string{
	// Images.
	".gif",
	".png",
	".jpeg",
	".jpg",
	".svg",

	// Fonts.
	".woff2",
	".woff",
	".ttf",
	".eot",

	// Assets.
	".pdf",

	// Stylesheets & scripts.
	".css",
	".js",
}

func Handler(domain config.Domain) http.HandlerFunc {
	log.WithFields(log.Fields{
		"hostname":             domain.Hostname,
		"service":              domain.Service,
		"reject-static-assets": domain.RejectStaticAssets,
		"virtual-hostname":     domain.VirtualHostname,
		"cors-origins":         domain.CORS.Origins,
	}).Info("Domain configured")

	if len(domain.Paths) == 0 {
		domain.Paths = append(domain.Paths, config.Path{
			Match:   "/",
			Service: domain.Service,
		})
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Rechaza ficheros estáticos si se ha activado la configuración.
		if domain.RejectStaticAssets && collections.HasString(assetsExts, filepath.Ext(r.URL.Path)) {
			http.Error(w, "Asset Not Found", http.StatusNotFound)
			return
		}

		start := time.Now()

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

		var path config.Path
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
			"latency-ms":    int64(time.Since(start) / time.Millisecond),
			"resp-size":     length,
			"status":        resp.StatusCode,
		}).Info("request")
	}
}

type transport struct{}

func (transport *transport) RoundTrip(r *http.Request) (*http.Response, error) {
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
