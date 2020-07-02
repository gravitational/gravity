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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/process"
	gcfg "github.com/gravitational/gravity/lib/processconfig"

	yaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/olekukonko/tablewriter"
)

func statusSite() error {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			}}}
	targetURL := defaults.GravityServiceURL + "/healthz"
	re, err := client.Get(targetURL)
	if err != nil {
		return trace.Wrap(err, "failed connecting to %v", targetURL)
	}
	defer re.Body.Close()
	if re.StatusCode == http.StatusOK {
		fmt.Printf("site is up and running\n")
		return nil
	}
	out, _ := ioutil.ReadAll(re.Body)
	return trace.ConnectionProblem(nil, fmt.Sprintf("got response: %v %v", targetURL, string(out)))
}

// startSite starts cluster controller
func startSite(configDir, importDir string) error {
	return process.Run(context.TODO(), configDir, importDir, process.NewProcess)
}

// initCluster imports site state from the specified import directory
func initCluster(configDir, importDir string) error {
	cfg, teleportCfg, err := gcfg.ReadConfig(configDir)
	if err != nil {
		return trace.Wrap(err)
	}

	p, err := process.New(context.TODO(), *cfg, *teleportCfg)
	if err != nil {
		return trace.Wrap(err)
	}

	err = p.ImportState(importDir)
	if err != nil {
		return trace.Wrap(err)
	}

	err = p.InitRPCCredentials()
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

func listSites(env *localenv.LocalEnvironment, opsCenterURL string) error {
	operator, err := env.OperatorService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}
	account, err := ops.UpsertSystemAccount(operator)
	if err != nil {
		return trace.Wrap(err)
	}
	siteList, err := operator.GetSites(account.ID)
	if err != nil {
		return trace.Wrap(err)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Status", "App"})

	var data [][]string
	for _, s := range siteList {
		data = append(data, []string{
			s.Domain,
			s.State,
			s.App.Package.String(),
		})
	}

	table.AppendBulk(data)
	table.Render()
	return nil
}

func getClusterReport(env *localenv.LocalEnvironment, targetFile string, since time.Duration) error {
	f, err := os.Create(targetFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	report, err := operator.GetSiteReport(ops.GetClusterReportRequest{
		SiteKey: ops.SiteKey{
			AccountID:  site.AccountID,
			SiteDomain: site.Domain,
		},
		Since: since,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer report.Close()

	if _, err := io.Copy(f, report); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("report for %v exported to %v\n", site, targetFile)
	return nil
}

// ClusterInfo collects information about the local cluster
type ClusterInfo struct {
	// App contains the information about the application running in cluster
	App AppInfo `json:"app"`
	// Endpoints is cluster's application endpoints
	Endpoints []ops.Endpoint `json:"endpoints"`
}

// AppInfo contains the information about the application running on local cluster
type AppInfo struct {
	// Name is the app name
	Name string `json:"name"`
	// Version is the app version
	Version string `json:"version"`
}

// GetLocalClusterInfo returns information about the local cluster
func GetLocalClusterInfo(env *localenv.LocalEnvironment) (*ClusterInfo, error) {
	operator, err := env.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endpoints, err := operator.GetApplicationEndpoints(cluster.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ClusterInfo{
		App: AppInfo{
			Name:    cluster.App.Manifest.Metadata.Name,
			Version: cluster.App.Manifest.Metadata.ResourceVersion,
		},
		Endpoints: endpoints,
	}, nil
}

func printLocalClusterInfo(env *localenv.LocalEnvironment, outFormat constants.Format) error {
	info, err := GetLocalClusterInfo(env)
	if err != nil {
		return trace.Wrap(err)
	}
	switch outFormat {
	case constants.EncodingText, constants.EncodingYAML:
		bytes, err := yaml.Marshal(info)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	case constants.EncodingJSON:
		bytes, err := json.Marshal(info)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(bytes))
	default:
		return trace.BadParameter("unknown output format: %s", outFormat)
	}
	return nil
}

func completeInstallerStep(env *localenv.LocalEnvironment, supportAction string) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	req := ops.CompleteFinalInstallStepRequest{
		AccountID:  cluster.AccountID,
		SiteDomain: cluster.Domain,
	}
	err = operator.CompleteFinalInstallStep(req)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Cluster %v installer step has been marked completed.\n", cluster.Domain)
	return nil
}

func resetPassword(env *localenv.LocalEnvironment) error {
	operator, err := env.OperatorService(defaults.GravityServiceURL)
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	user, err := operator.GetLocalUser(ops.SiteKey{
		AccountID:  site.AccountID,
		SiteDomain: site.Domain,
	})
	if err != nil {
		if trace.IsNotFound(err) {
			fmt.Printf("couldn't find local user for %v\n", site.Domain)
			return nil
		}
		return trace.Wrap(err)
	}

	password, err := operator.ResetUserPassword(ops.ResetUserPasswordRequest{
		AccountID:  site.AccountID,
		SiteDomain: site.Domain,
		Email:      user.GetName(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("password for %v has been reset to: %v\n", user.GetName(), password)
	return nil
}

func getLocalSite(env *localenv.LocalEnvironment) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%s", site.Domain)
	return nil
}

const stateResetWarning = `WARNING! This operation will force-set the cluster state to active without any
extra checks.

If used improperly, it may lead to inconsistent cluster state which may affect
future operations, so only proceed if you're certain of what you're doing.

Before resetting the cluster state consider doing the following:

 * Inspect "gravity status" to understand which state the cluster is in.

 * If there're unfinished operations, use "gravity plan" commands to properly
   complete or roll them back.

 * Refer to https://gravitational.com/gravity/docs/cluster/#managing-operations
   for more information about operation management.
`

func resetClusterState(env *localenv.LocalEnvironment, confirmed bool) error {
	if !confirmed {
		env.Println(color.YellowString(stateResetWarning))
		if err := enforceConfirmation("Proceed?"); err != nil {
			return trace.Wrap(err)
		}
	}

	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	err = operator.ActivateSite(ops.ActivateSiteRequest{
		AccountID:  site.AccountID,
		SiteDomain: site.Domain,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("Cluster %s state has been set to active\n", site.Domain)
	return nil
}

func stepDown(env *localenv.LocalEnvironment) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	err = operator.StepDown(site.Key())
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("current active master has been asked to step down\n")
	return nil
}
