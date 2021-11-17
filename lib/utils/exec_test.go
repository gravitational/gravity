/*
Copyright 2021 Gravitational, Inc.

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

package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/gravitational/gravity/lib/run"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

func TestOutputRace(t *testing.T) {
	g, ctx := run.WithContext(context.Background())
	for i := 0; i < 10; i++ {
		g.Go(ctx, func() error {
			cmd := helperCommandContext(ctx)
			var out bytes.Buffer
			logger := logrus.WithField(trace.Component, t.Name())
			return ExecL(cmd, &out, logger)
		})
	}
	_ = g.Wait()
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("TEST_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	msg := "toomanyrequests: You have reached your pull rate limit. You may increase the limit by authenticating and upgrading: https://www.example.com/increase-rate-limit"
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintln(os.Stdout, msg)
}

func helperCommandContext(ctx context.Context, s ...string) (cmd *exec.Cmd) {
	cs := []string{"-test.run=TestHelperProcess", "--"}
	cs = append(cs, s...)
	if ctx != nil {
		cmd = exec.CommandContext(ctx, os.Args[0], cs...)
	} else {
		cmd = exec.Command(os.Args[0], cs...)
	}
	cmd.Env = append(os.Environ(), "TEST_WANT_HELPER_PROCESS=1")
	return cmd
}
