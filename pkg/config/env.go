package config

import (
	"os"
)

func IsLocal() bool {
	return Version() == ""
}

func Version() string {
	return os.Getenv("VERSION")
}
