/*
Copyright 2019 Gravitational, Inc.

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

package disks

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/gravitational/trace"
)

// EtcdSpec returns fio job spec suitable for testing etcd device.
func EtcdSpec(path string, duration time.Duration) ([]byte, error) {
	vars := map[string]string{
		"filename": path,
		"duration": fmt.Sprintf("%vs", duration.Seconds()),
	}
	var spec bytes.Buffer
	if err := etcdSpecTemplate.Execute(&spec, vars); err != nil {
		return nil, trace.Wrap(err)
	}
	return spec.Bytes(), nil
}

var (
	// etcdSpecTemplate is the fio job template used to test etcd disk.
	//
	// See inline comments for the test details.
	etcdSpecTemplate = template.Must(template.New("etcd.job").Parse(`[etcd]
# perform sequential writes
rw=write
# use write() syscall for writes
ioengine=sync
# sync every data write to disk
fdatasync=1
# test file, should reside where etcd WAL will be
filename={{.filename}}
# average block size written by etcd
bs=2300
# total size of the test file
size=22m
# limit total runtime so the test doesn't take too long
runtime={{.duration}}
`))
)
