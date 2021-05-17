package magnet

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gravitational/trace"
)

// DefaultVersion generates a default version string from git.
func DefaultVersion() string {
	longTag, err := Output(context.TODO(), "git", "describe", "--tags", "--dirty")
	if err != nil {
		panic(fmt.Sprint("failed to fetch git version:\n", trace.DebugReport(err)))
	}

	return longTag
}

// DefaultHash retrieves the git hash that can be embedded within the binary as build information.
func DefaultHash() string {
	hash, err := Output(context.TODO(), "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		panic(fmt.Sprint("failed to fetch git hash:\n", trace.DebugReport(err)))
	}

	return hash
}

// DefaultLogDir is a default relative path to place logs for this particular build.
func DefaultLogDir() string {
	return "build/logs"
}

// DefaultBuildDir is a default location to place build artifacts at build/<version>.
func DefaultBuildDir(version string) string {
	return filepath.Join("build/", version)
}
