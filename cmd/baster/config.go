package main

import (
  "time"

  log "github.com/sirupsen/logrus"
  "github.com/juju/errors"

  "baster/pkg/external/k8s"
  "baster/pkg/config"
)

var lastVersion string

func detectConfigChange() (*config.Config, bool, error) {
  client, err := k8s.NewPodClient()
  if err != nil {
    return nil, false, errors.Trace(err)
  }

  for {
    cm, err := client.GetConfigMap("baster")
    if err != nil {
      log.WithFields(log.Fields{"error": err}).Errorf("cannot get baster configmap, retrying in 15 seconds")
      time.Sleep(15 * time.Second)
      continue
    }

    if cm == nil || cm.Data["config.hcl"] == "" {
      log.Errorf("no configmap with the name baster and a config.hcl file found, retrying in 15 seconds")
      time.Sleep(15 * time.Second)
      continue
    }

    if cm.Metadata.ResourceVersion == lastVersion {
      time.Sleep(1 * time.Minute)
      continue
    }

    cnf, err := config.Load(cm.Data["config.hcl"])
    if err != nil {
      log.WithFields(log.Fields{"error": err}).Errorf("configuration is not valid, retrying in 15 seconds")
      time.Sleep(15 * time.Second)
      continue
    }

    if err := cnf.IsValid(); err != nil {
      log.WithFields(log.Fields{"error": err}).Errorf("configuration is not valid, retrying in 15 seconds")
      time.Sleep(15 * time.Second)
      continue
    }

    break
  }

  lastVersion = cnf.Version
  return cnf, true, nil
}
