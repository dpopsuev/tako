package fold

import (
	"fmt"
	"runtime/debug"
	"time"
)

// buildLDFlags generates -ldflags for embedding version information
// into the fold-generated binary. Sets version, commit hash, build time,
// and origami framework version.
func buildLDFlags(manifestPath string) string {
	version := "dev"
	commit := "unknown"
	buildTime := time.Now().UTC().Format(time.RFC3339)
	takoVersion := "unknown"

	// Read origami module version from build info.
	if info, ok := debug.ReadBuildInfo(); ok {
		takoVersion = info.Main.Version
		if takoVersion == "(devel)" {
			takoVersion = "dev"
		}
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 { //nolint:mnd // short hash
				commit = s.Value[:7]
			}
		}
	}

	// Use manifest filename as version hint.
	if manifestPath != "" {
		version = fmt.Sprintf("fold-%s", commit)
	}

	return fmt.Sprintf(
		"-X main.Version=%s -X main.Commit=%s -X main.BuildTime=%s -X main.OrigamiVersion=%s",
		version, commit, buildTime, takoVersion,
	)
}
