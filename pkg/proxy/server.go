package proxy

import (
	"crypto/tls"
	"net/http"

	"github.com/golang/crypto/acme/autocert"
	"github.com/juju/errors"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"

	"github.com/altipla-consulting/baster/pkg/config"
	"github.com/altipla-consulting/baster/pkg/monitoring"
	"github.com/altipla-consulting/baster/pkg/stores"
)

type Server struct {
	monitors []monitoring.Interface
	store    autocert.Cache
	hs       hostSwitch

	manager            *autocert.Manager
	managerHTTPHandler http.Handler

	version string
}

type hostSwitch map[string]http.Handler

func (hs hostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler := hs[r.Host]; handler != nil {
		handler.ServeHTTP(w, r)
	} else {
		http.Error(w, "hostname not found", http.StatusNotFound)
	}
}

func NewServer(settings *config.Settings) (*Server, error) {
	server := &Server{
		hs: make(hostSwitch),
	}

	logger := log.WithField("version", settings.Version)
	logger.Info("Configure new server")

	if settings.ACME.IsActive() && !config.IsLocal() {
		logger.WithField("email", settings.ACME.Email).Info("Configure ACME account")

		cache, err := stores.NewDatastore()
		if err != nil {
			return nil, errors.Trace(err)
		}

		var whitelist []string
		for _, domain := range settings.Domains {
			whitelist = append(whitelist, domain.Hostname)
		}

		server.manager = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Email:      settings.ACME.Email,
			Cache:      cache,
			HostPolicy: autocert.HostWhitelist(whitelist...),
		}
	}

	if settings.Redirects.Apply != "" {
		logger.WithField("endpoint", settings.Redirects.Apply).Info("External redirects enabled")
	}
	if settings.Auth.Endpoint != "" {
		logger.WithField("endpoint", settings.Auth.Endpoint).Info("External auth enabled")
	}

	if settings.Monitoring.BigQuery.IsActive() {
		logger.WithFields(log.Fields{
			"dataset": settings.Monitoring.BigQuery.Dataset,
			"table":   settings.Monitoring.BigQuery.Table,
		}).Info("Configure BigQuery monitoring")

		m, err := monitoring.NewBigQuery(settings)
		if err != nil {
			return nil, errors.Trace(err)
		}
		server.monitors = append(server.monitors, m)
	}

	for _, domain := range settings.Domains {
		f := log.Fields{
			"name":     domain.Name,
			"hostname": domain.Hostname,
			"service":  domain.Service,
		}
		if domain.VirtualHostname != "" {
			f["virtual-hostname"] = domain.VirtualHostname
		}
		if len(domain.CORS.Origins) > 0 {
			f["cors-origins"] = domain.CORS.Origins
		}
		if domain.Redirect != "" {
			f["redirect"] = domain.Redirect
		}
		if len(domain.HopHeaders) > 0 {
			f["hop-headers"] = domain.HopHeaders
		}
		logger.WithFields(f).Info("Configure domain")

		r := httprouter.New()
		r.GET("/health", healthHandler)
		r.NotFound = http.HandlerFunc(proxyHandler(settings, domain, server.monitors))

		server.hs[domain.Hostname] = r
	}

	return server, nil
}

func (server *Server) SecureHandler() http.Handler {
	return server.hs
}

func (server *Server) InsecureHandler() (http.Handler, error) {
	if server.manager == nil {
		return nil, errors.New("HTTPS not configured properly")
	}

	r := httprouter.New()
	r.GET("/health", healthHandler)
	r.NotFound = http.HandlerFunc(redirectHandler)

	return server.manager.HTTPHandler(r), nil
}

func (server *Server) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if server.manager == nil {
		return nil, errors.New("HTTPS not configured properly")
	}

	return server.manager.GetCertificate(hello)
}
