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

package system

import (
	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

// Chroot executes chroot syscall on the specified directory
func Chroot(path string) error {
	if err := unix.Chroot(path); err != nil {
		return trace.ConvertSystemError(err)
	}

	if err := unix.Chdir("/"); err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}
