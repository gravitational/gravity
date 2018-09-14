/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
