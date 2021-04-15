// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gofrs/flock"
	"github.com/gravitational/trace"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

// defaultReposFile is the default repositories file. This file contains repository
// names and URLs.
var defaultReposFile = helmpath.ConfigPath("repositories.yaml")

type repoAddOptions struct {
	name      string
	url       string
	username  string
	password  string
	repoFile  string
	repoCache string
}

// repoAdd adds a chart repository. Code ported from:
// https://github.com/helm/helm/blob/v3.4.2/cmd/helm/repo_add.go
func (o *repoAddOptions) repoAdd() error {
	// Ensure the file directory exists as it is required for file locking
	err := os.MkdirAll(filepath.Dir(o.repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return trace.Wrap(err)
	}

	// Acquire a file lock for process synchronization
	fileLock := flock.New(strings.Replace(o.repoFile, filepath.Ext(o.repoFile), ".lock", 1))
	lockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock()
	}
	if err != nil {
		return trace.Wrap(err)
	}

	bytes, err := ioutil.ReadFile(o.repoFile)
	if err != nil && !os.IsNotExist(err) {
		return trace.Wrap(err)
	}

	var repoFile repo.File
	if err := yaml.Unmarshal(bytes, &repoFile); err != nil {
		return trace.Wrap(err)
	}

	entry := repo.Entry{
		Name:     o.name,
		URL:      o.url,
		Username: o.username,
		Password: o.password,
	}

	repository, err := repo.NewChartRepository(&entry, getter.All(cli.New()))
	if err != nil {
		return trace.Wrap(err)
	}

	if o.repoCache != "" {
		repository.CachePath = o.repoCache
	}

	if _, err := repository.DownloadIndexFile(); err != nil {
		return trace.Wrap(err, "looks like %q is not a valid chart repository or cannot be reached", o.url)
	}

	repoFile.Update(&entry)

	if err := repoFile.WriteFile(o.repoFile, defaults.SharedReadMask); err != nil {
		return trace.Wrap(err)
	}

	log.WithField("repo", o.name).Info("Repo has been added to your repositories.")
	return nil
}

type repoRemoveOptions struct {
	names     []string
	repoFile  string
	repoCache string
}

// repoRemove removes one or more chart repositories. Returns without any changes
// if no repositories are configured. Code ported from:
// https://github.com/helm/helm/blob/v3.4.2/cmd/helm/repo_remove.go
func (o *repoRemoveOptions) repoRemove() error {
	repoFile, err := repo.LoadFile(o.repoFile)
	if os.IsNotExist(errors.Cause(err)) || len(repoFile.Repositories) == 0 {
		log.Debug("No repositories configured.")
		return nil
	}

	if err != nil {
		return trace.Wrap(err)
	}

	for _, name := range o.names {
		if !repoFile.Remove(name) {
			log.WithField("repo", name).Debug("Repo was not found.")
			continue
		}
		if err := repoFile.WriteFile(o.repoFile, defaults.SharedReadMask); err != nil {
			return trace.Wrap(err)
		}
		if err := removeRepoCache(o.repoCache, name); err != nil {
			return trace.Wrap(err)
		}
		log.WithField("repo", name).Info("Repo has been removed from your repositories.")
	}

	return nil
}

func removeRepoCache(root, name string) error {
	chartsFile := filepath.Join(root, helmpath.CacheChartsFile(name))
	if _, err := os.Stat(chartsFile); err == nil {
		if err := os.Remove(chartsFile); err != nil {
			log.WithField("charts-file", chartsFile).Debug("Failed to remove charts file.")
		}
	}

	indexFile := filepath.Join(root, helmpath.CacheIndexFile(name))
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return trace.Wrap(err, "unable to remove index file %s", indexFile)
	}
	return trace.Wrap(os.Remove(indexFile))
}
