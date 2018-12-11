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
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

// DropCapabilities drops all capabilities except those specified with keep
// from the current process
func DropCapabilities(keep map[int]struct{}) error {
	maxCap, err := maxCap()
	if err != nil {
		return trace.Wrap(err)
	}
	const minCap int = 0

	for cap := minCap; cap <= maxCap; cap++ {
		if _, exists := keep[cap]; exists {
			continue
		}

		if err := unix.Prctl(unix.PR_CAPBSET_READ, uintptr(cap), 0, 0, 0); err != nil {
			if errno, ok := err.(syscall.Errno); ok && errno == unix.EINVAL {
				break
			}
			return trace.ConvertSystemError(err)
		}

		if err := unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(cap), 0, 0, 0); err != nil {
			// ignore EINVAL since the capability may not be supported
			if errno, ok := err.(syscall.Errno); ok && errno == unix.EINVAL {
				continue
			} else if errno, ok := err.(syscall.Errno); ok && errno == unix.EPERM {
				return trace.AccessDenied("missing CAP_SETPCAP capability")
			}
			return trace.ConvertSystemError(err)
		}
	}

	return nil
}

// maxCap returns the integer corresponding to the highest known
// capability on this system.
func maxCap() (cap int, err error) {
	// contains a single integer followed by a newline
	f, err := os.Open("/proc/sys/kernel/cap_last_cap")
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}
	defer f.Close()

	buf := make([]byte, 16)
	n, err := f.Read(buf)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}

	if n >= 16 {
		return 0, trace.BadParameter("cap_last_cap too long: %d", n)
	}
	return strconv.Atoi(strings.TrimSpace(string(buf[:n])))
}

// CAP_SYS_CHROOT defines the number of the linux capability to execute chroot
const CAP_SYS_CHROOT = 18
