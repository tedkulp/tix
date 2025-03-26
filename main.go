package main

import (
	"fmt"
	"os"

	"github.com/tedkulp/tix/cmd"
	"github.com/tedkulp/tix/internal/logger"
)

func main() {
	// The logger will be properly initialized in cmd.Execute() via PersistentPreRun
	// Ensure cmd.Execute() is called first before any logging
	if err := cmd.Execute(); err != nil {
		logger.Error("Command execution failed", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
