package cli

import (
	"os"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"

	"github.com/gravitational/trace"
)

// ensureDirectories checks to see if $HELM_HOME exists.
//
// If $HELM_HOME does not exist, this function will create it.
//
// This method was adopted from Helm:
//
// https://github.com/helm/helm/blob/v2.12.0/cmd/helm/init.go#L348
func ensureDirectories(home helmpath.Home) error {
	configDirectories := []string{
		home.String(),
		home.Repository(),
		home.Cache(),
		home.LocalRepository(),
		home.Plugins(),
		home.Starters(),
		home.Archive(),
	}
	for _, path := range configDirectories {
		if fi, err := os.Stat(path); err != nil {
			log.Infof("Creating %v.", path)
			if err := os.MkdirAll(path, defaults.SharedDirMask); err != nil {
				return trace.ConvertSystemError(err)
			}
		} else if !fi.IsDir() {
			return trace.BadParameter("%v must be a directory", path)
		}
	}
	return nil
}

// ensureReposFile returns the repositories file.
//
// If the repositories file does not exist, an empty one is initialized.
func ensureReposFile(path string) (*repo.RepoFile, error) {
	_, err := utils.StatFile(path)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		log.Infof("Creating %v.", path)
		err := repo.NewRepoFile().WriteFile(path, defaults.SharedReadMask)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return repo.LoadRepositoriesFile(path)
}
