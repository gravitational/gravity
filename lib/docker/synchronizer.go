package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/distribution"
	dockerref "github.com/docker/distribution/reference"
	regclient "github.com/docker/distribution/registry/client"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/run"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// RegistryInfo contains information about connecting to a registry.
type RegistryInfo struct {
	// Address stores the address of the registry as host:port
	Address string
	// Protocol stores the Protocol (https or http)
	Protocol string
}

// GetURL returns the url for current registry
func (i *RegistryInfo) GetURL() string {
	return fmt.Sprintf("%s://%s", i.Protocol, i.Address)
}

// NewSynchronizer creates a new Synchronizer
func NewSynchronizer(log log.FieldLogger, dockerClient Interface, progressReporter utils.Progress) *Synchronizer {
	return &Synchronizer{
		log:              log,
		dockerPuller:     NewPuller(dockerClient),
		dockerClient:     dockerClient,
		progressReporter: progressReporter,
	}
}

// Synchronizer contains the logic for pulling and exporting image layers
type Synchronizer struct {
	log              log.FieldLogger
	dockerPuller     *Puller
	dockerClient     Interface
	progressReporter utils.Progress
}

// PushImage pushes the specified image into the registry
func (h *Synchronizer) PushImage(image, registryAddr string) error {
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

func (h *Synchronizer) tagCmd(image string, tag loc.DockerImage) error {
	opts := dockerapi.TagImageOptions{
		Repo:  fmt.Sprintf("%v/%v", tag.Registry, tag.Repository),
		Tag:   tag.Tag,
		Force: true,
	}
	h.log.Infof("Tagging %v with opts=%v.", image, opts)
	return trace.Wrap(h.dockerClient.TagImage(image, opts))
}

func (h *Synchronizer) pushCmd(image loc.DockerImage) error {
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
func (h *Synchronizer) ImageExists(ctx context.Context, registryURL, repository, tag string) (bool, error) {
	refName, err := dockerref.WithName(repository)
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
func (h *Synchronizer) PullAndExportImages(ctx context.Context, images []string, reg RegistryInfo, forcePull bool, parallel int) error {
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

func (h *Synchronizer) pullAndExportImageIfNeeded(ctx context.Context, image string, reg RegistryInfo, forcePull bool) error {
	if forcePull {
		return h.pullAndPush(image, reg, true)
	}
	exists, err := h.checkImageInRegistry(ctx, image, reg)
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

func (h *Synchronizer) checkImageInRegistry(ctx context.Context, image string, reg RegistryInfo) (bool, error) {
	parsedImage, err := loc.ParseDockerImage(image)
	if err != nil {
		return false, trace.Wrap(err)
	}
	exists, err := h.ImageExists(ctx, reg.GetURL(), parsedImage.Repository, parsedImage.Tag)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return exists, nil
}

func (h *Synchronizer) pullAndPush(image string, reg RegistryInfo, needPull bool) error {
	if needPull {
		err := h.dockerPuller.Pull(image)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(h.PushImage(image, reg.Address))
}

// ImageTags returns the list of tags for specified image from the registry
func (h *Synchronizer) ImageTags(ctx context.Context, registryURL, repository string) ([]string, error) {
	refName, err := dockerref.WithName(repository)
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

func (h *Synchronizer) removeTagCmd(tag loc.DockerImage) error {
	localImage := tag.String()
	h.log.Infof("Removing %v.", localImage)
	return h.dockerClient.RemoveImage(localImage)
}
