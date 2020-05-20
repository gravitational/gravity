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

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/localenv"

	"github.com/gravitational/trace"
)

type appSyncConfig struct {
	// Image is an application image path or locator.
	Image string
	// registryConfig is configuration of a registry to push images to.
	registryConfig
}

// registryConfig describes Docker registry configuration.
type registryConfig struct {
	// Registry is a registry address.
	Registry string
	// CAPath is CA certificate path for a registry.
	CAPath string
	// CertPath is a client certificate path for a registry.
	CertPath string
	// KeyPath is a client key path for a registry.
	KeyPath string
	// Username is optional username for basic auth.
	Username string
	// Password is optional password for basic auth.
	Password string
	// Prefix is optional registry prefix when pushing images.
	Prefix string
	// Insecure indicates insecure registry.
	Insecure bool
	// ScanningRepository is a docker repository to push a copy of all vendored images
	// Used internally so the registry can scan those images and report on vulnerabilities
	ScanningRepository *string
	// ScanningTagPrefix is a prefix to add to each tag when pushed to help identify the image from the scan results
	ScanningTagPrefix *string
}

// imageService returns a new registry client for this config.
func (c registryConfig) imageService() (docker.ImageService, error) {
	req := docker.RegistryConnectionRequest{
		RegistryAddress: c.Registry,
		CACertPath:      c.CAPath,
		ClientCertPath:  c.CertPath,
		ClientKeyPath:   c.KeyPath,
		Username:        c.Username,
		Password:        c.Password,
		Prefix:          c.Prefix,
		Insecure:        c.Insecure,
	}

	if c.ScanningRepository != nil {
		return docker.NewScanningImageService(req, docker.ScanConfig{
			RemoteRepository: *c.ScanningRepository,
			TagPrefix:        *c.ScanningTagPrefix,
		})
	}

	return docker.NewImageService(req)
}

func appSync(env *localenv.LocalEnvironment, conf appSyncConfig) error {
	imageEnv, err := localenv.NewImageEnvironment(localenv.ImageEnvironmentConfig{
		Path: conf.Image,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return appSyncEnv(env, imageEnv, conf)
}

func appSyncEnv(env *localenv.LocalEnvironment, imageEnv *localenv.ImageEnvironment, conf appSyncConfig) error {
	if err := httplib.InGravity(env.DNS.Addr()); err == nil {
		// If we're running inside Gravity cluster, sync application images
		// to all cluster registries and push the application package to
		// the local cluster.
		log.Info("Detected Gravity cluster.")
		cluster, err := env.LocalCluster()
		if err != nil {
			return trace.Wrap(err)
		}
		registries, err := getRegistries(context.TODO(), env, cluster.ClusterState.Servers)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, registry := range registries {
			env.PrintStep("Pushing application images to Docker registry %v", registry)
			imageService, err := docker.NewClusterImageService(registry)
			if err != nil {
				return trace.Wrap(err)
			}
			err = service.SyncApp(context.TODO(), service.SyncRequest{
				PackService:  imageEnv.Packages,
				AppService:   imageEnv.Apps,
				ImageService: imageService,
				Package:      imageEnv.Manifest.Locator(),
				Progress:     env,
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}
		env.PrintStep("Pushing application image to local cluster")
		clusterPackages, err := env.ClusterPackages()
		if err != nil {
			return trace.Wrap(err)
		}
		clusterApps, err := env.SiteApps()
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = service.PullApp(service.AppPullRequest{
			SrcPack: imageEnv.Packages,
			DstPack: clusterPackages,
			SrcApp:  imageEnv.Apps,
			DstApp:  clusterApps,
			Package: imageEnv.Manifest.Locator(),
			Upsert:  true,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	} else if httplib.InKubernetes() {
		// If we're running inside generic Kubernetes cluster, sync images
		// to the registry specified on the command line.
		log.Info("Detected generic Kubernetes cluster.")
		env.PrintStep("Pushing application images to Docker registry %v", conf.Registry)
		imageService, err := conf.imageService()
		if err != nil {
			return trace.Wrap(err)
		}

		err = service.SyncApp(context.TODO(), service.SyncRequest{
			PackService:  imageEnv.Packages,
			AppService:   imageEnv.Apps,
			ImageService: imageService,
			Package:      imageEnv.Manifest.Locator(),
			Progress:     env,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		return trace.BadParameter("not inside a Kubernetes cluster")
	}
	return nil
}
