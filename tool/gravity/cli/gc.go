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

	"github.com/gravitational/gravity/lib/app/docker"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/vacuum"
	"github.com/gravitational/gravity/lib/vacuum/prune"
	"github.com/gravitational/gravity/lib/vacuum/prune/journal"
	"github.com/gravitational/gravity/lib/vacuum/prune/pack"
	"github.com/gravitational/gravity/lib/vacuum/prune/registry"

	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/sirupsen/logrus"
)

func garbageCollect(env *localenv.LocalEnvironment, manual, confirmed bool) error {
	if !confirmed {
		env.Println("This operation will also remove docker images that " +
			"you manually pushed to the docker registry. Are you sure?")
		resp, err := confirm()
		if err != nil {
			return trace.Wrap(err)
		}
		if !resp {
			env.Println("Action cancelled by user.")
			return nil
		}
	}

	collector, err := newCollector(env)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx := context.TODO()
	if !manual {
		err = collector.Run(ctx)
		return trace.Wrap(err)
	}

	err = collector.Create(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	env.Println(`
The garbage collection operation has been created in manual mode.

To view the operation plan, run:

$ gravity plan

To perform the collection, execute each phase in the order it appears in
the plan by running:

$ sudo gravity plan execute --phase=<phase-id>

To resume automatic collection from any point, run:

$ gravity plan resume`)
	return nil
}

func newCollector(env *localenv.LocalEnvironment) (*vacuum.Collector, error) {
	clusterPackages, err := env.ClusterPackages()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterApps, err := env.SiteApps()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operator, err := env.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportClient, err := env.TeleportClient(constants.Localhost)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a teleport client")
	}

	proxy, err := teleportClient.ConnectToProxy(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err, "failed to connect to teleport proxy")
	}

	key, err := operator.CreateClusterGarbageCollectOperation(
		ops.CreateClusterGarbageCollectOperationRequest{
			AccountID:   cluster.AccountID,
			ClusterName: cluster.Domain,
		},
	)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotImplemented(
				"cluster operator does not implement the API required for garbage collection. " +
					"Please make sure you're running the command on a compatible cluster.")
		}
		return nil, trace.Wrap(err)
	}

	defer func() {
		r := recover()
		triggered := err == nil && r == nil
		if !triggered {
			if errDelete := operator.DeleteSiteOperation(*key); errDelete != nil {
				log.Warnf("Failed to clean up garbage collection operation %v: %v.",
					key, trace.DebugReport(errDelete))
			}
		}
		if r != nil {
			panic(r)
		}
	}()

	operation, err := operator.GetSiteOperation(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	remoteApps, err := collectRemoteApplications(operator, cluster.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	runtimePath, err := getRuntimePackagePath(env.Packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if clusterEnv.Client == nil {
		return nil, trace.BadParameter("this operation can only be executed on one of the master nodes")
	}

	ctx := context.TODO()
	req := deployAgentsRequest{
		clusterState: cluster.ClusterState,
		cluster:      *cluster,
		clusterEnv:   clusterEnv,
		proxy:        proxy,
		servers:      cluster.ClusterState.Servers,
		version:      version.Get().Version,
	}
	creds, err := deployAgents(ctx, env, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)

	collector, err := vacuum.New(vacuum.Config{
		App: &storage.Application{
			Locator:  cluster.App.Package,
			Manifest: cluster.App.Manifest,
		},
		RemoteApps:    remoteApps,
		Apps:          clusterApps,
		Packages:      clusterPackages,
		LocalPackages: env.Packages,
		Operator:      operator,
		Operation:     operation,
		Servers:       cluster.ClusterState.Servers,
		ClusterKey:    cluster.Key(),
		RuntimePath:   runtimePath,
		Silent:        env.Silent,
		Runner:        runner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collector, nil
}

func executeGarbageCollectPhaseForOperation(env *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	clusterPackages, err := env.ClusterPackages()
	if err != nil {
		return trace.Wrap(err)
	}

	clusterApps, err := env.SiteApps()
	if err != nil {
		return trace.Wrap(err)
	}

	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	runtimePath, err := getAnyRuntimePackagePath(env.Packages)
	if err != nil {
		return trace.Wrap(err, "failed to fetch the path to the container's rootfs")
	}

	creds, err := rpc.ClientCredentials(clusterPackages)
	if err != nil {
		return trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)

	collector, err := vacuum.New(vacuum.Config{
		App: &storage.Application{
			Locator:  cluster.App.Package,
			Manifest: cluster.App.Manifest,
		},
		Apps:          clusterApps,
		Packages:      clusterPackages,
		LocalPackages: env.Packages,
		Operator:      operator,
		Operation:     &operation,
		Servers:       cluster.ClusterState.Servers,
		ClusterKey:    cluster.Key(),
		RuntimePath:   runtimePath,
		Silent:        env.Silent,
		Runner:        runner,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = collector.RunPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func removeUnusedImages(env *localenv.LocalEnvironment, dryRun, confirmed bool) error {
	if !dryRun && !confirmed {
		env.Println("This operation will also remove docker images that " +
			"you manually pushed to the docker registry on this node. Are you sure?")
		resp, err := confirm()
		if err != nil {
			return trace.Wrap(err)
		}
		if !resp {
			env.Println("Action cancelled by user.")
			return nil
		}
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}

	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := clusterEnv.Operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: constants.LocalRegistryAddr,
		CertName:        constants.DockerRegistry,
		CACertPath:      state.Secret(stateDir, defaults.RootCertFilename),
		ClientCertPath:  state.Secret(stateDir, "kubelet.cert"),
		ClientKeyPath:   state.Secret(stateDir, "kubelet.key"),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	config := registry.Config{
		App:          &cluster.App.Package,
		Apps:         clusterEnv.Apps,
		Packages:     clusterEnv.Packages,
		ImageService: imageService,
		Config: prune.Config{
			DryRun:      dryRun,
			FieldLogger: logrus.WithField(trace.Component, "gc/registry"),
			Silent:      env.Silent,
		},
	}
	pruner, err := registry.New(config)
	if err != nil {
		return trace.Wrap(err)
	}

	err = pruner.Prune(context.TODO())
	return trace.Wrap(err)
}

func removeUnusedPackages(env *localenv.LocalEnvironment, dryRun, pruneClusterPackages bool) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	remoteApps, err := collectRemoteApplications(operator, cluster.Key())
	if err != nil {
		return trace.Wrap(err)
	}

	config := pack.Config{
		App: &storage.Application{
			Locator:  cluster.App.Package,
			Manifest: cluster.App.Manifest,
		},
		Apps:     remoteApps,
		Packages: env.Packages,
		Config: prune.Config{
			DryRun:      dryRun,
			FieldLogger: logrus.WithField(trace.Component, "gc:registry"),
			Silent:      env.Silent,
		},
	}
	pruner, err := pack.New(config)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx := context.TODO()
	err = pruner.Prune(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if !pruneClusterPackages {
		return nil
	}

	clusterPackages, err := env.ClusterPackages()
	if err != nil {
		return trace.Wrap(err)
	}

	config.Packages = clusterPackages
	pruner, err = pack.New(config)
	if err != nil {
		return trace.Wrap(err)
	}

	err = pruner.Prune(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func collectRemoteApplications(operator ops.Operator, clusterKey ops.SiteKey) (remoteApps []storage.Application, err error) {
	accounts, err := operator.GetAccounts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, account := range accounts {
		clusters, err := operator.GetSites(account.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, remoteCluster := range clusters {
			if remoteCluster.AccountID == clusterKey.AccountID &&
				remoteCluster.Domain == clusterKey.SiteDomain {
				continue
			}
			remoteApps = append(remoteApps, storage.Application{
				Locator:  remoteCluster.App.Package,
				Manifest: remoteCluster.App.Manifest,
			})
		}
	}
	return remoteApps, nil
}

func removeUnusedJournalFiles(env *localenv.LocalEnvironment, machineIDFile, logDir string) (err error) {
	if machineIDFile == "" {
		machineIDFile = defaults.SystemdMachineIDFile
	}
	if logDir == "" {
		logDir = defaults.SystemdLogDir
	}

	pruner, err := journal.New(journal.Config{
		LogDir:        logDir,
		MachineIDFile: machineIDFile,
		Config: prune.Config{
			FieldLogger: logrus.WithField(trace.Component, "gc:journal"),
			Silent:      env.Silent,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = pruner.Prune(context.TODO())
	return trace.Wrap(err)
}
