/*
Copyright 2017 Gravitational, Inc.

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

package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// SysctlCHecker verifies various /proc filesystem runtime parameters
type SysctlChecker struct {
	// Name is checker name
	CheckerName string
	// Param is parameter name
	Param string
	// Expected is expected parameter value
	Expected string
	// OnMissing is description when parameter is missing
	OnMissing string
	// OnValueMismatch is description when parameter value is not equal to expected
	OnValueMismatch string
	// SkipNotFound will skip reporting a sysctl that is not found on the system
	SkipNotFound bool
}

// Name returns name of checker
func (c *SysctlChecker) Name() string {
	return c.CheckerName
}

// Check will verify the parameter value is as expected or complain otherwise
func (c *SysctlChecker) Check(ctx context.Context, reporter health.Reporter) {
	value, err := Sysctl(c.Param)

	if err == nil && value == c.Expected {
		reporter.Add(&pb.Probe{
			Checker: c.CheckerName,
			Status:  pb.Probe_Running,
		})
		return
	}

	data := SysctlCheckerData{ParameterName: c.Param, ParameterValue: c.Expected}
	bytes, marshalErr := json.Marshal(data)
	if marshalErr != nil {
		reporter.Add(NewProbeFromErr(c.CheckerName, fmt.Sprintf(
			"failed to marshal %v", data), trace.Wrap(marshalErr)))
	}

	if err == nil && value != c.Expected {
		reporter.Add(&pb.Probe{
			Checker:     c.CheckerName,
			Detail:      c.OnValueMismatch,
			Status:      pb.Probe_Failed,
			CheckerData: bytes,
		})
		return
	}

	if trace.IsNotFound(err) {
		if !c.SkipNotFound {
			reporter.Add(&pb.Probe{
				Checker:     c.CheckerName,
				Detail:      c.OnMissing,
				Status:      pb.Probe_Failed,
				Error:       trace.UserMessage(err),
				CheckerData: bytes,
			})
		}
		return
	}

	reporter.Add(NewProbeFromErr(c.CheckerName,
		fmt.Sprintf("failed to query sysctl parameter %s", c.Param), trace.Wrap(err)))
}

// Sysctl returns kernel parameter by reading proc/sys
func Sysctl(name string) (string, error) {
	path := filepath.Clean(filepath.Join("/proc", "sys", strings.Replace(name, ".", "/", -1)))
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	if len(data) == 0 {
		return "", trace.BadParameter("empty output from sysctl")
	}
	return string(data[:len(data)-1]), nil
}

// SysctlCheckerData gets attached to the sysctl parameter check probes
type SysctlCheckerData struct {
	// ParameterName is the name of sysctl parameter
	ParameterName string
	// ParameterValue is the expected value of sysctl parameter
	ParameterValue string
}
