package proxy

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"

	"github.com/altipla-consulting/baster/pkg/config"
)

type ServerWatcher struct {
	mx     *sync.RWMutex // protects server
	server *Server

	secureHandler   http.Handler
	insecureHandler http.Handler

	lastVersion string
}

func NewServerWatcher() (*ServerWatcher, error) {
	settings, err := config.ParseSettings()
	if err != nil {
		return nil, errors.Trace(err)
	}

	server, err := NewServer(settings)
	if err != nil {
		return nil, errors.Trace(err)
	}

	w := &ServerWatcher{
		mx:          new(sync.RWMutex),
		server:      server,
		lastVersion: settings.Version,
	}

	go w.bgWatch()

	return w, nil
}

func (server *ServerWatcher) bgWatch() {
	for {
		if err := server.bgUpdate(); err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
				"stack": errors.ErrorStack(err),
			}).Error("Server background update failed")
		}

		time.Sleep(30 * time.Second)
	}
}

func (w *ServerWatcher) bgUpdate() error {
	settings, err := config.ParseSettings()
	if err != nil {
		return errors.Trace(err)
	}

	if settings.Version == w.lastVersion {
		return nil
	}

	// Cambia la versión lo primero. En caso de error evita que se repita una
	// y otra vez el intento de cambiar a la nueva versión provocando errores.
	// Si el cambio va mal una vez no lo vuelve a intentar de nuevo.
	w.mx.Lock()
	w.lastVersion = settings.Version
	w.mx.Unlock()

	server, err := NewServer(settings)
	if err != nil {
		return errors.Trace(err)
	}

	secureHandler := w.server.SecureHandler()

	var insecureHandler http.Handler
	if !config.IsLocal() {
		insecureHandler, err = w.server.InsecureHandler()
		if err != nil {
			return errors.Trace(err)
		}
	}

	w.mx.Lock()
	w.server = server
	w.secureHandler = secureHandler
	w.insecureHandler = insecureHandler
	w.mx.Unlock()

	return nil
}

func (watcher *ServerWatcher) SecureServeHTTP(w http.ResponseWriter, r *http.Request) {
	watcher.mx.RLock()
	secureHandler := watcher.secureHandler
	watcher.mx.RUnlock()

	secureHandler.ServeHTTP(w, r)
}

func (watcher *ServerWatcher) InsecureServeHTTP(w http.ResponseWriter, r *http.Request) {
	watcher.mx.RLock()
	insecureHandler := watcher.insecureHandler
	watcher.mx.RUnlock()

	insecureHandler.ServeHTTP(w, r)
}

func (w *ServerWatcher) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	w.mx.RLock()
	server := w.server
	w.mx.RUnlock()

	return server.GetCertificate(hello)
}
