package config

import (
	"os"
)

func IsDebug() bool {
	return os.Getenv("DEBUG") == "true"
}
