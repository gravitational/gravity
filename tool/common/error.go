package common

import (
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// ProcessRunError looks at the error that happened during a CLI command
// execution and converts it to a user-friendly format
func ProcessRunError(runErr error) error {
	if runErr == nil {
		return nil
	}
	switch err := trace.Unwrap(runErr).(type) {
	case *utils.UnsupportedFilesystemError:
		return trace.BadParameter("state directory %[1]q resides on an unsupported "+
			"filesystem. Typically this happens when using a shared folder "+
			"(e.g. vboxsf) or other filesystem that does not support mmap. Make "+
			"sure that %[1]q is located on the local filesystem / block device "+
			"or specify custom state directory using --state-dir flag", err.Path)
	}
	return runErr
}
