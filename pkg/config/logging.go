package config

import (
	log "github.com/sirupsen/logrus"
)

func init() {
	if IsLocal() {
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
		log.SetLevel(log.DebugLevel)
	}
}
