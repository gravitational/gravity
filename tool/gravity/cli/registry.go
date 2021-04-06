/*
Copyright 2019 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/buger/goterm"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

type registryConnectionRequest struct {
	address  string
	caPath   string
	certPath string
	keyPath  string
}

func (r *registryConnectionRequest) checkAndSetDefaults(env *localenv.LocalEnvironment) (err error) {
	if r.address == "" {
		r.address, err = utils.ResolveAddr(env.DNS.Addr(), defaults.DockerRegistry)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		r.address = utils.EnsurePort(r.address, strconv.Itoa(defaults.DockerRegistryPort))
	}
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	if r.caPath == "" {
		r.caPath = state.Secret(stateDir, defaults.RegistryCAFilename)
	}
	if r.certPath == "" {
		r.certPath = state.Secret(stateDir, defaults.RegistryCertFilename)
	}
	if r.keyPath == "" {
		r.keyPath = state.Secret(stateDir, defaults.RegistryKeyFilename)
	}
	return nil
}

// listRegistryContents displays images from the specified Docker registry.
func listRegistryContents(ctx context.Context, env *localenv.LocalEnvironment, req registryConnectionRequest, format constants.Format) error {
	if err := req.checkAndSetDefaults(env); err != nil {
		return trace.Wrap(err)
	}
	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: req.address,
		CACertPath:      req.caPath,
		ClientCertPath:  req.certPath,
		ClientKeyPath:   req.keyPath,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	images, err := imageService.List(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	switch format {
	case constants.EncodingText:
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		common.PrintTableHeader(t, []string{"Image", "Tags"})
		for _, image := range images {
			fmt.Fprintf(t, "%v\t%v\n", image.Repository, strings.Join(image.Tags, ", "))
		}
		fmt.Println(t.String())
	case constants.EncodingJSON:
		bytes, err := json.Marshal(images)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	case constants.EncodingYAML:
		bytes, err := yaml.Marshal(images)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	default:
		return trace.BadParameter("unsupported output format %q, supported are %v",
			format, constants.OutputFormats)
	}
	return nil
}
