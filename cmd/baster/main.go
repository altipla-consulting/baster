package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
)

func main() {
	if err := run(); err != nil {
	  log.Fatal(errors.ErrorTrace(err))
	}
}

func run() error {
	return nil
}
