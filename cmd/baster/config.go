package main

import (
	"flag"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"github.com/naoina/toml"
)

var (
	configPath = flag.String("config", "/etc/baster/config.toml", "Configuration file")
)

type Config struct {
	Service []Service
}

type Service struct {
	Name       string
	K8sService string
	Hostname   string
	Route      []Route
}

type Route struct {
	URL        string
	K8sService string
}

func LoadConfig() (*Config, error) {
	log.WithFields(log.Fields{"path": *configPath}).Info("load config file")

	f, err := os.Open(*configPath)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	cnf := new(Config)
	if err := toml.NewDecoder(f).Decode(cnf); err != nil {
		return nil, errors.Trace(err)
	}

	return cnf, nil
}
