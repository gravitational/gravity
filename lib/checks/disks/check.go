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
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// Check executes the disk performance test using fio tool with the provided
// job spec file and returns the test results in json format.
func Check(ctx context.Context, spec []byte) ([]byte, error) {
	// The test expects to find fio tool in the node's temporary directory
	// as it was supposed to be placed there by the operation's init phase.
	fioPath := filepath.Join(os.TempDir(), constants.FioBin)
	if _, err := utils.StatFile(fioPath); err != nil {
		return nil, trace.Wrap(err)
	}
	// Write the job spec to a temporary file.
	jobPath := filepath.Join(os.TempDir(), "fio.job")
	if err := ioutil.WriteFile(jobPath, spec, defaults.SharedReadMask); err != nil {
		return nil, trace.Wrap(err)
	}
	defer os.Remove(jobPath)
	// Execute the job.
	var out bytes.Buffer
	cmd := []string{fioPath, jobPath, "--output-format", "json"}
	if err := utils.RunStream(ctx, &out, cmd...); err != nil {
		return nil, trace.Wrap(err)
	}
	return out.Bytes(), nil
}
