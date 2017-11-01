package config

import (
	"time"

	"github.com/hashicorp/hcl"
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"

	"baster/pkg/external/k8s"
)

func AutoUpdate(updates chan *Config) {
	k8sclient, err := k8s.NewPodClient()
	if err != nil {
		log.WithFields(log.Fields{"error": errors.ErrorStack(err)}).Fatalf("cannot initialize the kubernetes client")
		return
	}

	var lastVersion string
	for {
		cm, err := k8sclient.GetConfigMap("baster")
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("cannot contact kubernetes cluster, retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}

		if cm == nil || cm.Data["config.hcl"] == "" {
			log.Error("cannot find baster configmap with a config.hcl file, retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}

		if lastVersion == cm.Metadata.ResourceVersion {
			time.Sleep(1 * time.Minute)
			continue
		}

		cnf := new(Config)
		if err := hcl.Decode(cnf, cm.Data["config.hcl"]); err != nil {
			log.WithFields(log.Fields{"error": err}).Error("configuration is not valid, retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}
		if err := cnf.IsValid(); err != nil {
			log.WithFields(log.Fields{"error": err}).Error("configuration is not valid, retrying in 15 seconds")
			time.Sleep(15 * time.Second)
			continue
		}

		log.Infof("config updated from version %s to %s of baster configmap", lastVersion, cm.Metadata.ResourceVersion)
		lastVersion = cm.Metadata.ResourceVersion

		updates <- cnf
	}
}
