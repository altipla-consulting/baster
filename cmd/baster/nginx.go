package main

import (
	"io/ioutil"
	"os/exec"

	"github.com/juju/errors"

	"baster/pkg/nginx"
	"baster/pkg/config"
)

var nginxStarted bool

func reloadNginxConfig(cnf *config.Config) error {
	nginxConf, err := nginx.GenerateConfig(cnf)
	if err != nil {
	  return errors.Trace(err)
	}

	if err := ioutil.WriteFile("/etc/nginx/nginx.conf", nginxConf, 0600); err != nil {
	  return errors.Trace(err)
	}

	if nginxStarted {
		out, err := exec.Command("nginx", "-s", "reload").Output()
		if err != nil {
		  return errors.Trace(err)
		}
		log.WithFields(log.Fields{"output": out}).Info("nginx reloaded")
	} else {
		out, err := exec.Command("nginx").Output()
		if err != nil {
		  return errors.Trace(err)
		}
		log.WithFields(log.Fields{"output": out}).Info("nginx started")

		nginxStarted = true
	}

	return nil
}
