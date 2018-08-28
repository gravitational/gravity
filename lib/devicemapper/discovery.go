package devicemapper

import (
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
)

// GetSystemDirectory determines the location of the LVM system directory.
func GetSystemDirectory() (string, error) {
	systemDir := os.Getenv(constants.LVMSystemDirEnvvar)
	if systemDir != "" {
		return systemDir, nil
	}

	isDir, err := utils.IsDirectory(constants.LVMSystemDir)
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	if err == nil && isDir {
		return constants.LVMSystemDir, nil
	}
	return "", trace.NotFound("no LVM system directory found")
}
