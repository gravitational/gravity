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
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	upgradechecks "github.com/gravitational/gravity/lib/update/cluster/checks"

	"github.com/fatih/color"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

type preflightChecksConfig struct {
	manifestPath string
	imagePath    string
	profileName  string
	autoFix      bool
	timeout      time.Duration
}

func executePreflightChecks(env *localenv.LocalEnvironment, config preflightChecksConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()

	err := localenv.DetectCluster(env)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if trace.IsNotFound(err) {
		env.PrintStep("No deployed cluster detected, running install preflight checks")
		return checkInstall(ctx, env, config)
	}

	env.PrintStep("Detected deployed cluster, running upgrade preflight checks")
	return checkUpgrade(ctx, env, config)
}

func checkInstall(ctx context.Context, env *localenv.LocalEnvironment, config preflightChecksConfig) error {
	data, err := ioutil.ReadFile(config.manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}
	manifest, err := schema.ParseManifestYAML(data)
	if err != nil {
		return trace.Wrap(err)
	}
	profileName := config.profileName
	if profileName == "" {
		profileName, err = manifest.FirstNodeProfileName()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	env.PrintStep("Running checks against node profile %q", profileName)
	result, err := checks.ValidateLocal(checks.LocalChecksRequest{
		Context:  ctx,
		Manifest: *manifest,
		Role:     profileName,
		AutoFix:  config.autoFix,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(result.Failed)+len(result.Fixable) == 0 {
		env.PrintStep(color.GreenString("Checks have succeeded!"))
		return nil
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

func checkUpgrade(ctx context.Context, env *localenv.LocalEnvironment, config preflightChecksConfig) error {
	tarballEnv, err := localenv.NewTarballEnvironment(localenv.TarballEnvironmentArgs{
		StateDir: config.imagePath,
	})
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
	packages, err := env.ClusterPackages()
	if err != nil {
		return trace.Wrap(err)
	}
	manifest, err := schema.ParseManifest(config.manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}
	// Need to upload gravity package from the upgrade image, otherwise
	// RPC agents may fail to deploy because they will be looking for
	// this gravity package in the cluster's package service.
	err = uploadGravity(ctx, env, manifest, tarballEnv.Packages, packages)
	if err != nil {
		return trace.Wrap(err)
	}
	// Deploy RPC agents that will be used for running checks on the nodes.
	credentials, err := rpcAgentDeployHelper(ctx, env, deployOptions{})
	if err != nil {
		return trace.Wrap(err)
	}
	checker, err := upgradechecks.NewChecker(ctx, upgradechecks.CheckerConfig{
		ClusterOperator: operator,
		ClusterApps:     apps,
		UpgradeApps:     tarballEnv.Apps,
		UpgradePackage:  manifest.Locator(),
		Agents:          fsm.NewAgentRunner(credentials),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Running upgrade checks for cluster image %v:%v",
		manifest.Metadata.Name, manifest.Metadata.ResourceVersion)
	checksErr := checker.Run(ctx)
	if err := rpcAgentShutdown(env); err != nil {
		log.WithError(err).Error("Failed to shutdown agents.")
	}
	if checksErr != nil {
		return trace.Wrap(checksErr)
	}
	env.PrintStep(color.GreenString("Checks have succeeded!"))
	return nil
}

// uploadGravity uploads gravity package from the source to the destination.
func uploadGravity(ctx context.Context, env *localenv.LocalEnvironment, manifest *schema.Manifest, src, dst pack.PackageService) error {
	gravityPackage, err := manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Uploading package %v:%v to the local cluster",
		gravityPackage.Name, gravityPackage.Version)
	puller := &app.Puller{
		SrcPack: src,
		DstPack: dst,
		Upsert:  true,
	}
	err = puller.PullPackage(ctx, *gravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func printFailedChecks(failed []*pb.Probe) {
	if len(failed) == 0 {
		return
	}
	fmt.Printf("Failed checks:\n")
	fmt.Printf(checks.FormatFailedChecks(failed))
}
