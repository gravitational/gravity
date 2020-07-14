package magnet

import (
	"context"

	"github.com/gravitational/trace"
)

func Version() string {
	longTag, err := Output(context.TODO(), "git", "describe", "--tags", "--dirty")
	if err != nil {
		panic(trace.DebugReport(err))
	}

	return longTag
}

func Hash() string {
	hash, err := Output(context.TODO(), "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		panic(trace.DebugReport(err))
	}

	return hash
}
