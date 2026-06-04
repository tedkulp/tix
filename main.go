package main

import (
	"os"

	"github.com/tedkulp/tix/cmd"
)

func main() {
	// The logger will be properly initialized in cmd.Execute() via PersistentPreRun
	// Ensure cmd.Execute() is called first before any logging
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
