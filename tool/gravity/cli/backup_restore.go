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
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/hooks"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	dockerarchive "github.com/docker/docker/pkg/archive"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
)

func backup(env *localenv.LocalEnvironment, tarball string, timeout time.Duration, follow, silent bool) (err error) {
	ctx := context.Background()
	// if we're streaming logs to stdout, no much sense in showing our progress indicator
	noProgress := silent || follow
	progress := utils.NewProgress(ctx, "backup", 2, noProgress)
	defer progress.Stop()
	progress.NextStep("backing up to %v", tarball)
	return runBackupRestore(env, "backup",
		func(env *localenv.LocalEnvironment, backupPath string, req *app.HookRunRequest) error {
			req.Hook = schema.HookBackup
			if timeout != 0 {
				req.Timeout = timeout
			}
			apps, err := env.SiteApps()
			if err != nil {
				return trace.Wrap(err)
			}
			ref, err := app.StreamAppHook(
				ctx, apps, *req, getStreamingWriter(silent, follow))
			if err != nil {
				return trace.Wrap(err)
			}
			defer func() {
				err := apps.DeleteAppHookJob(ctx, app.DeleteAppHookJobRequest{
					HookRef: *ref,
				})
				if err != nil {
					log.Warningf("failed to delete hook %v: %v",
						ref, trace.DebugReport(err))
				}
				if err = os.RemoveAll(backupPath); err != nil {
					log.Errorf("failed to remove backup directory %s: %v", backupPath, err)
				}
			}()
			err = compressDirectory(backupPath, tarball)
			if err != nil {
				return trace.Wrap(err)
			}
			progress.NextStep("backup is written to %v", tarball)
			return nil
		})
}

func restore(env *localenv.LocalEnvironment, tarball string, timeout time.Duration, follow, silent bool) error {
	ctx := context.Background()
	// if we're streaming logs to stdout, no much sense in showing our progress indicator
	noProgress := silent || follow
	progress := utils.NewProgress(ctx, "restore", 2, noProgress)
	defer progress.Stop()
	progress.NextStep("restoring from %v", tarball)
	return runBackupRestore(env, "restore",
		func(env *localenv.LocalEnvironment, backupPath string, req *app.HookRunRequest) error {
			f, err := os.Open(tarball)
			if err != nil {
				return trace.Wrap(err, "failed to open the tarball %q with backed up data", tarball)
			}
			defer f.Close()
			err = dockerarchive.Untar(f, backupPath, archive.DefaultOptions())
			if err != nil {
				return trace.Wrap(err)
			}
			defer func() {
				if err = os.RemoveAll(backupPath); err != nil {
					log.Errorf("failed to remove restore directory %s: %v", backupPath, err)
				}
			}()
			req.Hook = schema.HookRestore
			if timeout != 0 {
				req.Timeout = timeout
			}
			apps, err := env.SiteApps()
			if err != nil {
				return trace.Wrap(err)
			}
			_, err = app.StreamAppHook(ctx, apps, *req, getStreamingWriter(silent, follow))
			if err != nil {
				return trace.Wrap(err)
			}
			progress.NextStep("restored from %v", tarball)
			return nil
		})
}

func runBackupRestore(env *localenv.LocalEnvironment, operation string,
	fn func(env *localenv.LocalEnvironment, backupPath string, req *app.HookRunRequest) error) (err error) {

	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	node, err := findLocalServer(cluster.ClusterState.Servers)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("running %v for %v on %v", operation, cluster.App.Package, node.KubeNodeID())

	id, err := teleutils.CryptoRandomHex(3)
	if err != nil {
		return trace.Wrap(err, "failed to generate random ID")
	}

	req := &app.HookRunRequest{
		Application: cluster.App.Package,
		Volumes: []v1.Volume{{
			Name: hooks.VolumeBackup,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: fmt.Sprintf("/ext/state/%v/backup", id),
				},
			},
		}},
		VolumeMounts: []v1.VolumeMount{{
			Name:      hooks.VolumeBackup,
			MountPath: hooks.ContainerBackupDir,
		}},
		NodeSelector: map[string]string{
			v1.LabelHostname: node.KubeNodeID(),
		},
	}

	backupDir, err := localenv.InGravity(fmt.Sprintf("planet/state/%v/backup", id))
	if err != nil {
		return trace.Wrap(err)
	}

	err = fn(env, backupDir, req)
	return trace.Wrap(err)
}

func compressDirectory(dir, outputTarball string) error {
	archive, err := dockerarchive.Tar(dir, dockerarchive.Gzip)
	if err != nil {
		return trace.Wrap(err, "failed to compress the backup directory %v", dir)
	}
	defer archive.Close()
	f, err := os.Create(outputTarball)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	_, err = io.Copy(f, archive)
	return trace.Wrap(err)
}

// getStreamingWriter returns appropriate writer based on silent/follow flags
func getStreamingWriter(silent, follow bool) io.WriteCloser {
	if silent || !follow {
		return utils.NopWriteCloser(ioutil.Discard)
	}
	return utils.NopWriteCloser(os.Stdout)
}
