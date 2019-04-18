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

package webapi

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// webClusterInfo encapsulates basic information about cluster such as
// management endpoints and status used by the control panel.
type webClusterInfo struct {
	// ClusterState is the current cluster state.
	ClusterState string `json:"clusterState"`
	// PublicURLs is the advertised public cluster URLs set via auth gateway resource.
	PublicURLs []string `json:"publicURL"`
	// InternalURLs is a list of internal cluster management URLs.
	InternalURLs []string `json:"internalURLs"`
	// Commands contains various commands that can be run on the cluster.
	Commands webClusterCommands `json:"commands"`
}

// webClusterCommands contains commands displayed to a user for cluster
// expansion, remote access and so on.
type webClusterCommands struct {
	// TshLogin contains tsh login command.
	TshLogin string `json:"tshLogin"`
	// GravityDownload contains command to download gravity binary.
	GravityDownload string `json:"gravityDownload"`
	// GravityJoin contains gravity join commands for each node profile.
	GravityJoin map[string]string `json:"gravityJoin"`
}

// getClusterInfo collects information for the specified cluster.
func getClusterInfo(operator ops.Operator, cluster ops.Site) (*webClusterInfo, error) {
	authGateway, err := operator.GetAuthGateway(cluster.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	masterNode, err := cluster.FirstMaster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var internalAddrs []string
	for _, node := range cluster.Masters() {
		internalAddrs = append(internalAddrs, fmt.Sprintf("%v:%v",
			node.AdvertiseIP, defaults.GravitySiteNodePort))
	}
	var publicAddrs []string
	for _, webAddr := range authGateway.GetWebPublicAddrs() {
		publicAddrs = append(publicAddrs, utils.EnsurePort(
			webAddr, defaults.HTTPSPort))
	}
	var proxyAddr string
	if len(publicAddrs) != 0 {
		proxyAddr = publicAddrs[0]
	} else {
		proxyAddr = internalAddrs[0]
	}
	tshLoginCommand, err := renderCommand(tshLoginTpl, map[string]string{
		"proxyAddr": proxyAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	joinToken, err := operator.GetExpandToken(cluster.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gravityDownloadCommand, err := renderCommand(gravityDownloadTpl, map[string]string{
		"node":  masterNode.AdvertiseIP,
		"port":  strconv.Itoa(defaults.GravitySiteNodePort),
		"token": joinToken.Token,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gravityJoinCommands := make(map[string]string)
	for _, profile := range cluster.App.Manifest.NodeProfiles {
		gravityJoinCommands[profile.Name], err = renderCommand(gravityJoinTpl, map[string]string{
			"node":  masterNode.AdvertiseIP,
			"token": joinToken.Token,
			"role":  profile.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &webClusterInfo{
		ClusterState: cluster.State,
		PublicURLs:   makeURLs(publicAddrs),
		InternalURLs: makeURLs(internalAddrs),
		Commands: webClusterCommands{
			TshLogin:        tshLoginCommand,
			GravityDownload: gravityDownloadCommand,
			GravityJoin:     gravityJoinCommands,
		},
	}, nil
}

// renderCommand returns the rendered command based on provided template and parameters.
func renderCommand(tpl *template.Template, params map[string]string) (string, error) {
	var b bytes.Buffer
	if err := tpl.Execute(&b, params); err != nil {
		return "", trace.Wrap(err)
	}
	return b.String(), nil
}

// makeURLs converts provided addresses into URLs.
func makeURLs(addrs []string) (urls []string) {
	for _, addr := range addrs {
		if !strings.HasPrefix(addr, "https://") {
			urls = append(urls, fmt.Sprintf("https://%v", addr))
		} else {
			urls = append(urls, addr)
		}
	}
	return urls
}

var (
	// gravityJoinTpl is the gravity join command template.
	gravityJoinTpl = template.Must(template.New("join").Parse(
		"gravity join {{.node}} --token={{.token}} --role={{.role}}"))
	// gravityDownloadTpl is the gravity download command template.
	gravityDownloadTpl = template.Must(template.New("gravity").Parse(
		`curl -k -H "Authorization: Bearer {{.token}}" https://{{.node}}:{{.port}}/portal/v1/gravity -o gravity`))
	// tshLoginTpl is the tsh login command template.
	tshLoginTpl = template.Must(template.New("tsh").Parse(
		"tsh login --proxy={{.proxyAddr}}"))
)
