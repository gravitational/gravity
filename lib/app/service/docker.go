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

	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"
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
	s := docker.NewSynchronizer(log, dockerClient, progress)
	regInfo := docker.RegistryInfo{
		Address:  registry.Addr(),
		Protocol: "http",
	}

	err = s.PullAndExportImages(ctx, images, regInfo, forcePull, parallel)
	if err != nil {
		return trace.Wrap(err, "failed to export image layers")
	}

	return nil
}

// excludeImagesStartingWith excludes the items witch start with prefix
// It works with zero memory allocation and changes the input slice
func excludeImagesStartingWith(images []string, prefix string) []string {
	if prefix == "" {
		return images
	}
	x := 0
	for _, image := range images {
		if !strings.HasPrefix(image, prefix) {
			images[x] = image
			x++
		}
	}
	return images[:x]
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
