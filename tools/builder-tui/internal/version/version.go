// Package version exposes the binary's build identity. Set via -ldflags at
// build time; defaults are useful for `go run .`.
package version

var (
	// Version is the semver tag of the build (e.g. "v0.1.0").
	Version = "dev"
	// Commit is the git SHA the binary was built from.
	Commit = "unknown"
	// Date is the build timestamp (RFC3339).
	Date = "unknown"
)

// String returns a one-line build identity.
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
