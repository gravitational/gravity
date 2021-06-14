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
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	regclient "github.com/docker/distribution/registry/client"

	"github.com/gravitational/gravity/lib/docker"
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
	registryDir string,
	images []string,
	dockerClient docker.Interface,
	log log.FieldLogger,
	parallel int,
	forcePull bool,
	progress utils.Progress) error {

	config := docker.BasicConfiguration("127.0.0.1:0", registryDir)
	registry, err := docker.NewRegistry(config)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = registry.Start(); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if errStop := registry.Close(); errStop != nil {
			log.Warnf("Failed to stop registry: %v.", errStop)
		}
	}()
	helper := newDockerHelper(log, dockerClient, progress)
	regInfo := registryInfo{
		address:  registry.Addr(),
		protocol: "http",
	}
	err = helper.PullAndExportImages(ctx, images, regInfo, forcePull, parallel)
	if err != nil {
		return trace.Wrap(err, "failed to export image layers")
	}

	return nil
}

// registryInfo contains information about connecting to a registry.
type registryInfo struct {
	// address stores the address of the registry as host:port
	address string
	// protocol stores the protocol (https or http)
	protocol string
}

func (i *registryInfo) getURL() string {
	return fmt.Sprintf("%s://%s", i.protocol, i.address)
}

func newDockerHelper(log log.FieldLogger, dockerClient docker.Interface, progressReporter utils.Progress) *dockerHelper {
	return &dockerHelper{
		log:              log,
		dockerPuller:     docker.NewPuller(dockerClient),
		dockerClient:     dockerClient,
		progressReporter: progressReporter,
	}
}

// dockerHelper contains the logic for pulling and exporting image layers
type dockerHelper struct {
	log              log.FieldLogger
	dockerPuller     *docker.Puller
	dockerClient     docker.Interface
	progressReporter utils.Progress
}

// PushImage pushes the specified image into the registry
func (h *dockerHelper) PushImage(image, registryAddr string) error {
	parsedImage, err := loc.ParseDockerImage(image)
	if err != nil {
		return trace.Wrap(err)
	}
	dstDockerImage := loc.DockerImage{
		Registry:   registryAddr,
		Repository: parsedImage.Repository,
		Tag:        parsedImage.Tag,
	}
	if err = h.tagCmd(image, dstDockerImage); err != nil {
		return trace.Wrap(err)
	}
	if err = h.pushCmd(dstDockerImage); err != nil {
		h.log.Warnf("Failed to push %v: %v.", image, err)
		return trace.Wrap(err).AddField("image", image)
	}
	h.progressReporter.PrintSubStep("Vendored image %v", image)
	if err = h.removeTagCmd(dstDockerImage); err != nil {
		h.log.WithError(err).Debugf("Failed to remove %v.", image)
	}
	return nil
}

func (h *dockerHelper) tagCmd(image string, tag loc.DockerImage) error {
	opts := dockerapi.TagImageOptions{
		Repo:  fmt.Sprintf("%v/%v", tag.Registry, tag.Repository),
		Tag:   tag.Tag,
		Force: true,
	}
	h.log.Infof("Tagging %v with opts=%v.", image, opts)
	return trace.Wrap(h.dockerClient.TagImage(image, opts))
}

func (h *dockerHelper) pushCmd(image loc.DockerImage) error {
	opts := dockerapi.PushImageOptions{
		Name: fmt.Sprintf("%v/%v", image.Registry, image.Repository),
		Tag:  image.Tag,
	}
	h.log.Infof("Pushing %v.", opts)
	// Workaround a registry issue after updating go-dockerclient, set the password field to an invalid value so the
	// auth headers are set.
	// https://github.com/moby/moby/issues/10983
	return trace.Wrap(h.dockerClient.PushImage(opts, dockerapi.AuthConfiguration{
		Password: "not-a-real-password",
	}))
}

// ImageExists checks if the image exists in the registry
func (h *dockerHelper) ImageExists(ctx context.Context, registryURL, repository, tag string) (bool, error) {
	refName, err := reference.WithName(repository)
	if err != nil {
		return false, trace.Wrap(err)
	}

	rep, err := regclient.NewRepository(refName, registryURL, nil)
	if err != nil {
		return false, trace.Wrap(err)
	}

	manifestService, err := rep.Manifests(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	_, err = manifestService.Get(ctx, "", distribution.WithTag(tag))
	if err != nil {
		if strings.Contains(err.Error(), "manifest unknown") {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	return true, nil
}

// PullAndExportImages pulls and pushes the list of specified images into the registry
func (h *dockerHelper) PullAndExportImages(ctx context.Context, images []string, reg registryInfo, forcePull bool, parallel int) error {
	group, ctx := run.WithContext(ctx, run.WithParallel(parallel))
	for i := range images {
		image := images[i]
		group.Go(ctx, func() error {
			return h.pullAndExportImageIfNeeded(ctx, image, reg, forcePull)
		})
	}
	if err := group.Wait(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (h *dockerHelper) pullAndExportImageIfNeeded(ctx context.Context, image string, reg registryInfo, forcePull bool) error {
	if forcePull {
		return h.pullAndPush(image, reg, true)
	}
	parsedImage, err := loc.ParseDockerImage(image)
	if err != nil {
		return trace.Wrap(err)
	}
	exists, err := h.ImageExists(ctx, reg.getURL(), parsedImage.Repository, parsedImage.Tag)
	if err != nil {
		return trace.Wrap(err)
	}
	if exists {
		h.log.Infof("Skip pushing image %q. The image is already in the registry.", image)
		return nil
	}
	present, err := h.dockerPuller.IsImagePresent(image)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(h.pullAndPush(image, reg, !present))
}

func (h *dockerHelper) pullAndPush(image string, reg registryInfo, needPull bool) error {
	if needPull {
		err := h.dockerPuller.Pull(image)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(h.PushImage(image, reg.address))
}

// ImageTags returns the list of tags for specified image from the registry
func (h *dockerHelper) ImageTags(ctx context.Context, registryURL, repository string) ([]string, error) {
	refName, err := reference.WithName(repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rep, err := regclient.NewRepository(refName, registryURL, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	list, err := rep.Tags(ctx).All(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return list, nil
}

func (h *dockerHelper) removeTagCmd(tag loc.DockerImage) error {
	localImage := tag.String()
	h.log.Infof("Removing %v.", localImage)
	return h.dockerClient.RemoveImage(localImage)
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
func pullMissingRemoteImage(image string, puller docker.PullService, log log.FieldLogger, req VendorRequest) error {
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

func tagImageWithoutRegistry(image string, docker docker.Interface, log log.FieldLogger) error {
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
