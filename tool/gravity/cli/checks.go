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
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

func checkManifest(env *localenv.LocalEnvironment, manifestPath, imagePath, profileName string, autoFix bool) error {
	// If cluster is already deployed, run pre-upgrade checks.
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = operator.GetLocalSite()
	if err == nil {
		env.PrintStep("Detected deployed cluster, will run pre-upgrade checks")
		return checkUpgrade(context.TODO(), env, manifestPath, imagePath)
	}

	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}

	manifest, err := schema.ParseManifestYAML(data)
	if err != nil {
		return trace.Wrap(err)
	}

	result, err := checks.ValidateLocal(checks.LocalChecksRequest{
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

func checkUpgrade(ctx context.Context, env *localenv.LocalEnvironment, manifestPath, imagePath string) error {
	env.PrintStep("Deploying agents on the cluster nodes")
	credentials, err := rpcAgentDeploy(env, "", "")
	if err != nil {
		return trace.Wrap(err)
	}
	tarballEnv, err := localenv.NewTarballEnvironment(localenv.TarballEnvironmentArgs{})
	if err != nil {
		return trace.Wrap(err)
	}
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.SiteApps()
	if err != nil {
		return trace.Wrap(err)
	}
	manifest, err := schema.ParseManifest(manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}
	checker, err := ops.NewUpgradeChecker(ctx, ops.UpgradeCheckerConfig{
		ClusterOperator: operator,
		ClusterApps:     apps,
		UpgradeApps:     tarballEnv.Apps,
		UpgradePackage:  manifest.Locator(),
		Agents:          fsm.NewAgentRunner(credentials),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Running pre-flight checks for %v", manifest.Locator())
	checksErr := checker.Run(ctx)
	if err := rpcAgentShutdown(env); err != nil {
		log.WithError(err).Error("Failed to shutdown agents.")
	}
	if checksErr != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Checks have succeeded!")
	return nil
}

func printFailedChecks(failed []*pb.Probe) {
	if len(failed) == 0 {
		return
	}

	fmt.Printf("Failed checks:\n")
	fmt.Printf(checks.FormatFailedChecks(failed))
}
