// +build !selinux_embed

/*
Copyright 2020 Gravitational, Inc.

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

package policy

import (
	"net/http"
	"os"
	"time"
)

// Policy contains the SELinux policy.
var Policy http.FileSystem = newPolicyFS(http.Dir("assets"))

func newPolicyFS(fs http.FileSystem) policyFS {
	return policyFS{
		fs: fs,
	}
}

// Open returns a new http.File for the given name
func (r policyFS) Open(name string) (http.File, error) {
	f, err := r.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return policyFile{File: f}, nil
}

// policyFS wraps an existing http.FileSystem
type policyFS struct {
	fs http.FileSystem
}

// Stat returns an os.FileInfo that reports an empty modification time
func (r policyFile) Stat() (os.FileInfo, error) {
	s, err := r.File.Stat()
	if err != nil {
		// Returns original error on purpose
		return nil, err
	}
	return policyFileNoTime{FileInfo: s}, nil
}

// policyFile wraps an http.File to report an empty modification time
type policyFile struct {
	http.File
}

// ModTime returns the empty time stamp.
// Implements os.FileInfo
func (r policyFileNoTime) ModTime() time.Time {
	return time.Time{}
}

// policyFileNoTime is an os.FileInfo that always reports empty modification time
type policyFileNoTime struct {
	os.FileInfo
}
