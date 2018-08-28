package system

import (
	"bufio"
	"bytes"
	"context"
	"strings"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// GetFilesystem detects the filesystem on device specified with path
func GetFilesystem(ctx context.Context, path string, runner utils.CommandRunner) (filesystem string, err error) {
	var out bytes.Buffer
	err = runner.RunStream(&out, "lsblk", "--noheading", "--output", "FSTYPE", path)
	if err != nil {
		return "", trace.Wrap(err, "failed to determine filesystem type on %v", path)
	}

	s := bufio.NewScanner(&out)
	s.Split(bufio.ScanLines)

	for s.Scan() {
		// Return the first line of output
		return strings.TrimSpace(s.Text()), nil
	}
	if s.Err() != nil {
		return "", trace.Wrap(err)
	}

	return "", trace.NotFound("no filesystem found for %v", path)
}
