package version

// Version information
var (
	// Version is the current version of tix
	Version = "0.1.0"

	// Commit is the git commit hash
	Commit = "dev"

	// BuildDate is the date when the binary was built
	BuildDate = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns version information including commit hash and build date
func GetFullVersion() string {
	return Version + " (commit: " + Commit + ", built: " + BuildDate + ")"
}
