package keyval

import (
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

func (b *backend) GetApplication(repository, packageName, packageVersion string) (*storage.Package, error) {
	return b.GetPackage(repository, packageName, packageVersion)
}

func (b *backend) GetApplications(repository string, appType storage.AppType) ([]storage.Package, error) {
	var repos []string
	if repository != "" {
		repos = []string{repository}
	} else {
		out, err := b.GetRepositories()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, repo := range out {
			repos = append(repos, repo.GetName())
		}
	}
	var out []storage.Package
	for _, repo := range repos {
		packages, err := b.GetPackages(repo)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, app := range packages {
			if app.Manifest == nil {
				continue
			}
			if string(appType) != "" && app.Type != string(appType) {
				continue
			}
			out = append(out, app)
		}
	}
	return out, nil
}
