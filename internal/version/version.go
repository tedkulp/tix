package version

import (
	"fmt"
	"runtime"
)

// Version information
var (
	// Version is the current version of tix
	// This value is injected at build time by GoReleaser
	Version = "dev"

	// Commit is the git commit hash
	// This value is injected at build time by GoReleaser
	Commit = "none"

	// Date is the build date
	// This value is injected at build time by GoReleaser
	Date = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns version information including commit hash and build date
func GetFullVersion() string {
	return fmt.Sprintf("tix version %s (commit: %s, built: %s, %s/%s)",
		Version, Commit, Date, runtime.GOOS, runtime.GOARCH)
}
