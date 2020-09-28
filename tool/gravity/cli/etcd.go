/*
Copyright 2020 Gravitational, Inc.

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
	"fmt"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func etcdMigrate(fromVersion, toVersion string) (err error) {
	srcDir := getBaseEtcdDir(fromVersion)
	dstDir := getBaseEtcdDir(toVersion)
	log.WithFields(logrus.Fields{
		"from": srcDir,
		"to":   dstDir,
	}).Info("Copy etcd data.")
	return trace.Wrap(utils.CopyDirContents(srcDir, dstDir))
}

func getBaseEtcdDir(version string) (path string) {
	return filepath.Join(defaults.PlanetEtcdDir, fmt.Sprint("v", version))
}
