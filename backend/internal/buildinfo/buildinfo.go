package buildinfo

// Populated at link time via -ldflags -X.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Summary is a short single-line version for APIs and UI.
func Summary() string {
	if Version == "" || Version == "dev" {
		return "dev"
	}
	return Version
}
