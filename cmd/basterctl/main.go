package main

import (
	"os"
)

func main() {
	if err := CmdRoot.Execute(); err != nil {
		os.Exit(1)
	}
}
