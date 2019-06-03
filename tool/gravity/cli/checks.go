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

package cli

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/schema"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

func checkManifest(env *localenv.LocalEnvironment, manifestPath, profileName string, autoFix bool) error {
	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}

	manifest, err := schema.ParseManifestYAML(data)
	if err != nil {
		return trace.Wrap(err)
	}

	result, err := checks.ValidateLocal(context.TODO(), checks.LocalChecksRequest{
		Manifest: *manifest,
		Role:     profileName,
		AutoFix:  autoFix,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var failedErr, fixableErr error
	if len(result.Failed) > 0 {
		failedErr = trace.BadParameter(fmt.Sprintf("The following checks failed:\n%v",
			checks.FormatFailedChecks(result.Failed)))
	}
	if len(result.Fixable) > 0 {
		fixableErr = trace.BadParameter(fmt.Sprintf("The following checks failed, provide --autofix flag to let gravity to autofix them:\n%v",
			checks.FormatFailedChecks(result.Fixable)))
	}

	return trace.NewAggregate(failedErr, fixableErr)
}

func printFailedChecks(failed []*pb.Probe) {
	if len(failed) == 0 {
		return
	}

	fmt.Printf("Failed checks:\n")
	fmt.Printf(checks.FormatFailedChecks(failed))
}
