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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/gravitational/satellite/agent/health"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewScriptChecker returns a new script checker for the specified script.
// dir can specify an alternative working directory for the script.
// If workingDir is left unspecified, the running process's directory is used
// as a working directory.
func NewScriptChecker(script Script) health.Checker {
	return scriptChecker{Script: script}
}

// Name returns name of the checker.
// Implements health.Checker
func (r scriptChecker) Name() string {
	return scriptCheckerID
}

// Check creates a temporary file with the contents of r.Reader and executes
// it using shell.
func (r scriptChecker) Check(ctx context.Context, reporter health.Reporter) {
	f, err := ioutil.TempFile("", "monitoring")
	if err != nil {
		reporter.Add(NewProbeFromErr(r.Name(),
			fmt.Sprintf("failed to create script file in temporary directory %v", os.TempDir()),
			trace.ConvertSystemError(err)))
		return
	}
	defer f.Close()

	_, err = io.Copy(f, r.Reader)
	if err != nil {
		reporter.Add(NewProbeFromErr(r.Name(),
			fmt.Sprintf("failed to write script file %v", f.Name()),
			trace.Wrap(err)))
		return
	}

	args := append([]string{f.Name()}, r.Args...)
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Dir = r.WorkingDir
	buf, err := cmd.CombinedOutput()
	if err != nil {
		reporter.Add(NewProbeFromErr(r.Name(),
			fmt.Sprintf("script %q failed: %s", r.Description, buf),
			trace.Wrap(err)))
		return
	}

	log.Infof("Script %q: %s.", r.Description, buf)
	reporter.Add(NewSuccessProbe(r.Name()))
}

// Script defines a check based on a set of shell commands
type Script struct {
	// Reader specifies the contents of the script
	io.Reader
	// Description provides a desciption of the check script performs
	Description string
	// WorkingDir specifies the working directory for the script.
	// If unspecified, the script is executed in the working directory
	// of the calling process
	WorkingDir string
	// Args specifies the optional arguments to the script
	Args []string
}

// scriptChecker is a checker that executes the specified script
type scriptChecker struct {
	Script
}

const scriptCheckerID = "script-check"
