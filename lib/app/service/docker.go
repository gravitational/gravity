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

package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/app/docker"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/run"
	"github.com/gravitational/gravity/lib/utils"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// exportLayers exports the layers of the specified set of images into
// the specified local directory
func exportLayers(
	ctx context.Context,
	dir string,
	images []string,
	dockerClient docker.DockerInterface,
	log log.FieldLogger,
	parallel int, progress utils.Progress) error {
	layerExporter, err := newLayerExporter(dir, dockerClient, log, progress)
	if err != nil {
		return trace.Wrap(err, "failed to create layer export")
	}
	defer func() {
		if errStop := layerExporter.stop(); errStop != nil {
			log.Warnf("Failed to stop exporter: %v.", errStop)
		}
	}()

	if err = layerExporter.push(ctx, images, parallel); err != nil {
		return trace.Wrap(err, "failed to push images to local registry")
	}

	return nil
}

// newLayerExporter creates an instance of layer exporter
func newLayerExporter(exportDir string, client docker.DockerInterface, log log.FieldLogger, progress utils.Progress) (*layerExporter, error) {
	outputDir := filepath.Join(exportDir, defaults.RegistryDir)
	config := docker.BasicConfiguration("127.0.0.1:0", outputDir)
	registry, err := docker.NewRegistry(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = registry.Start(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &layerExporter{
		FieldLogger:      log,
		dockerClient:     client,
		registry:         registry,
		progressReporter: progress,
	}, nil
}

// layerExporter implements the logic of exporting layers of specified docker
// images to the given output directory using an instance of a temporary
// docker registry
type layerExporter struct {
	log.FieldLogger
	dockerClient     docker.DockerInterface
	registry         *docker.Registry
	progressReporter utils.Progress
}

// push pushes the list of specified images into the temporary local registry
func (r *layerExporter) push(ctx context.Context, images []string, parallel int) error {
	group, ctx := run.WithContext(ctx, run.WithParallel(parallel))
	for _, image := range images {
		group.Go(ctx, r.pushImage(image))
	}
	if err := group.Wait(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// stop stops the instance of the local registry
func (r *layerExporter) stop() error {
	return r.registry.Close()
}

func (r *layerExporter) pushImage(image string) func() error {
	return func() error {
		parsed, err := loc.ParseDockerImage(image)
		if err != nil {
			return trace.Wrap(err)
		}
		if err = r.tagCmd(image, parsed.Repository, parsed.Tag); err != nil {
			return trace.Wrap(err)
		}
		if err = r.pushCmd(parsed.Repository, parsed.Tag); err != nil {
			r.Warnf("Failed to push %v: %v.", image, err)
			return trace.Wrap(err)
		}
		r.progressReporter.PrintSubStep("Vendored image %v", image)
		if err = r.removeTagCmd(parsed.Repository, parsed.Tag); err != nil {
			r.Warnf("Failed to remove %v.", image)
		}
		return nil
	}
}

func (r *layerExporter) tagCmd(image, repository, tag string) error {
	opts := dockerapi.TagImageOptions{
		Repo:  fmt.Sprintf("%v/%v", r.registry.Addr(), repository),
		Tag:   tag,
		Force: true,
	}
	r.Infof("Tagging %v with opts=%v.", image, opts)
	return r.dockerClient.TagImage(image, opts)
}

func (r *layerExporter) pushCmd(name, tag string) error {
	opts := dockerapi.PushImageOptions{
		Name: fmt.Sprintf("%v/%v", r.registry.Addr(), name),
		Tag:  tag,
	}
	r.Infof("Pushing %v.", opts)
	return r.dockerClient.PushImage(opts, dockerapi.AuthConfiguration{})
}

func (r *layerExporter) removeTagCmd(name, tag string) error {
	if tag == "" {
		tag = "latest"
	}
	localImage := fmt.Sprintf("%v/%v:%v", r.registry.Addr(), name, tag)
	r.Infof("Removing %v.", localImage)
	return r.dockerClient.RemoveImage(localImage)
}

// parseImageNameTag parses the specified image reference into name/tag tuple.
// The returned name will include domain/path parts merged in a way to conform
// to telekube package name syntax.
// for input:
//	repo/subrepo/name:tag
// it returns
//	(repo-subrepo-name, tag)
func parseImageNameTag(image string) (name, tag string, err error) {
	ref, err := docker.Parse(image)
	if err != nil {
		return "", "", trace.Wrap(err, "failed to parse image reference %q", image)
	}
	named, isNamed := ref.(docker.Named)
	if !isNamed {
		return "", "", trace.BadParameter("image reference %v has no name", image)
	}
	if tagged, ok := ref.(docker.NamedTagged); ok {
		tag = tagged.Tag()
	}

	domain := docker.Domain(named)
	path := strings.Replace(docker.Path(named), "/", "-", -1)
	if domain != "" {
		host, port := utils.SplitHostPort(domain, "")
		if port != "" {
			host = fmt.Sprint(host, "-", port)
		}
		path = fmt.Sprint(host, "-", path)
	}

	return path, tag, nil
}

// pullMissingRemoteImages downloads a subset of remote images missing locally
func pullMissingRemoteImage(image string, puller docker.DockerPuller, log log.FieldLogger, req VendorRequest) error {
	log.Infof("Pulling: %s.", image)
	present, err := puller.IsImagePresent(image)
	if err != nil {
		return trace.Wrap(err)
	}
	if !present {
		req.ProgressReporter.PrintSubStep("Pulling remote image %v", image)
		return puller.Pull(image)
	}
	if req.Pull {
		req.ProgressReporter.PrintSubStep("Re-pulling remote image %v.", image)
		return puller.Pull(image)
	}
	req.ProgressReporter.PrintSubStep("Using local image %v", image)
	return nil
}

func tagImageWithoutRegistry(image string, docker docker.DockerInterface, log log.FieldLogger) error {
	// tag the image without a registry
	parsed, err := loc.ParseDockerImage(image)
	if err != nil {
		return trace.Wrap(err)
	}

	tagOpts := dockerapi.TagImageOptions{
		Repo:  parsed.Repository,
		Tag:   parsed.Tag,
		Force: true,
	}
	log.Infof("Tagging image %q: %#v.", image, tagOpts)

	if err = docker.TagImage(image, tagOpts); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
