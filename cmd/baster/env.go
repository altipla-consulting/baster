package main

import (
	"os"
)

func IsDebug() bool {
	return os.Getenv("DEBUG") == "true"
}
