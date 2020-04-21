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

package validation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"golang.org/x/net/context"
)

// CheckDisks executes the disk performance test using fio tool with the
// provided configuration and returns the test results.
func (r *Server) CheckDisks(ctx context.Context, req *proto.CheckDisksRequest) (*proto.CheckDisksResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	var jobs []*proto.FioJobResult
	for _, spec := range req.Jobs {
		jobRes, err := r.checkDisks(ctx, req.FioPath, spec)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		jobs = append(jobs, jobRes...)
	}
	return &proto.CheckDisksResponse{
		Jobs: jobs,
	}, nil
}

// checkDisks runs a single fio job.
func (r *Server) checkDisks(ctx context.Context, fioPath string, spec *proto.FioJobSpec) ([]*proto.FioJobResult, error) {
	// Make sure directory containing the test file exists and that it's
	// cleaned up after the test.
	if err := os.MkdirAll(filepath.Dir(spec.Filename), defaults.SharedDirMask); err != nil {
		return nil, trace.Wrap(err)
	}
	defer os.Remove(spec.Filename)
	var stdout, stderr bytes.Buffer
	cmd := append([]string{fioPath, "--output-format=json"}, spec.Flags()...)
	r.Infof("Running disk check: %v.", cmd)
	if err := utils.RunStream(ctx, &stdout, &stderr, cmd...); err != nil {
		return nil, trace.Wrap(err, "failed to execute fio test: %v", stderr.String())
	}
	r.Debugf("Disk check output: %v.", stdout.String())
	var res fioResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return nil, trace.Wrap(err, "failed to unmarshal fio result: %v", stdout.String())
	}
	return res.Jobs, nil
}

// fioResult describes a result of a fio test.
type fioResult struct {
	// Jobs is a list of executed jobs.
	Jobs []*proto.FioJobResult `json:"jobs"`
}
