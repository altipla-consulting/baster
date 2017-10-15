package config

import (
	"context"
	"sync"
	"time"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"
)

type Controller struct {
	*sync.RWMutex
	cnf *Config
}

func (ctrl *Controller) HostPolicy() autocert.HostPolicy {
	return func(ctx context.Context, host string) error {
		if _, service := ctrl.FindService(host); service == nil {
			return errors.Errorf("unknown service hostname: %s", host)
		}

		return nil
	}
}

func (ctrl *Controller) AutoUpdate() {
	log.Infof("using version %s of baster configmap", ctrl.cnf.Version)

	for {
		cnf, err := Load()
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("cannot check baster configmap")
			continue
		}

		if cnf == nil {
			log.Error("cannot find baster configmap, retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}

		if err := cnf.IsValid(); err != nil {
			log.WithFields(log.Fields{"error": err}).Error("configuration is not valid, retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}

		ctrl.RLock()
		if ctrl.cnf.Version == cnf.Version {
			ctrl.RUnlock()
			time.Sleep(1 * time.Minute)
			continue
		}
		ctrl.RUnlock()

		ctrl.Lock()
		ctrl.cnf = cnf
		ctrl.Unlock()
		log.Infof("config updated from version %s of baster configmap", cnf.Version)
		time.Sleep(1 * time.Minute)
	}
}

func (ctrl *Controller) FindService(hostname string) (string, *Service) {
	ctrl.RLock()
	defer ctrl.RUnlock()

	for name, service := range ctrl.cnf.Services {
		if service.Hostname == hostname {
			return name, service
		}
	}

	return "", nil
}
