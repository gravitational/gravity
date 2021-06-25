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

package pack

import (
	"io"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

func PackagesWithACL(packages PackageService, users users.Users, user storage.User, checker teleservices.AccessChecker) PackageService {
	return &ACLService{
		packages: packages,
		users:    users,
		user:     user,
		checker:  checker,
	}
}

// ACLService is permission aware service that wraps
// regular service and applies checks before every operation
type ACLService struct {
	packages PackageService
	users    users.Users
	user     storage.User
	checker  teleservices.AccessChecker
}

func (a *ACLService) context() *users.Context {
	return &users.Context{Context: teleservices.Context{User: a.user}}
}

func (a *ACLService) repoContext(repoName string) *users.Context {
	return &users.Context{
		Context: teleservices.Context{
			User:     a.user,
			Resource: storage.NewRepository(repoName),
		},
	}
}

func (a *ACLService) PortalURL() string {
	return a.packages.PortalURL()
}

func (a *ACLService) PackageDownloadURL(loc loc.Locator) string {
	return a.packages.PackageDownloadURL(loc)
}

func (a *ACLService) UpsertRepository(repository string, expires time.Time) error {
	if err := a.checker.CheckAccessToRule(a.repoContext(repository), teledefaults.Namespace, storage.KindRepository, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checker.CheckAccessToRule(a.repoContext(repository), teledefaults.Namespace, storage.KindRepository, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return a.packages.UpsertRepository(repository, expires)
}

// DeleteRepository deletes repository - packages will remain in the
// packages repository
func (a *ACLService) DeleteRepository(repository string) error {
	if err := a.checker.CheckAccessToRule(a.repoContext(repository), teledefaults.Namespace, storage.KindRepository, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return a.packages.DeleteRepository(repository)
}

// GetRepositories repositories returns a list of repositories
func (a *ACLService) GetRepositories() ([]string, error) {
	if err := a.checker.CheckAccessToRule(a.context(), teledefaults.Namespace, storage.KindRepository, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.packages.GetRepositories()
}

// GetRepository returns repository by name, returns error if it does not exist
func (a *ACLService) GetRepository(repository string) (storage.Repository, error) {
	if err := a.repoAction(repository, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.packages.GetRepository(repository)
}

func (a *ACLService) repoAction(repository string, verb string) error {
	// we have a higher level app kind, that grants access to default
	// repository,  with name defaults.SystemAccountOrg, check for it
	if repository == defaults.SystemAccountOrg {
		if err := a.checker.CheckAccessToRule(a.context(), teledefaults.Namespace, storage.KindApp, verb, false); err == nil {
			return nil
		}
	}
	return a.checker.CheckAccessToRule(a.repoContext(repository), teledefaults.Namespace, storage.KindRepository, verb, false)
}

// GetPackages returns a list of packages in repository
func (a *ACLService) GetPackages(repository string) ([]PackageEnvelope, error) {
	if err := a.repoAction(repository, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.packages.GetPackages(repository)
}

// CreatePackage creates package and adds it to to the existing repository
func (a *ACLService) CreatePackage(loc loc.Locator, data io.Reader, options ...PackageOption) (*PackageEnvelope, error) {
	if err := a.repoAction(loc.Repository, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.packages.CreatePackage(loc, data, options...)
}

// UpsertPackage creates package and adds it to to the existing repository
func (a *ACLService) UpsertPackage(loc loc.Locator, data io.Reader, options ...PackageOption) (*PackageEnvelope, error) {
	if err := a.repoAction(loc.Repository, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.repoAction(loc.Repository, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.packages.UpsertPackage(loc, data, options...)
}

// DeletePackage deletes package from all repositories
func (a *ACLService) DeletePackage(loc loc.Locator) error {
	if err := a.repoAction(loc.Repository, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.packages.DeletePackage(loc)
}

func (a *ACLService) UpdatePackageLabels(loc loc.Locator, addLabels map[string]string, removeLabels []string) error {
	if err := a.repoAction(loc.Repository, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return a.packages.UpdatePackageLabels(loc, addLabels, removeLabels)
}

// ReadPackage package opens and returns package contents
func (a *ACLService) ReadPackage(loc loc.Locator) (*PackageEnvelope, io.ReadCloser, error) {
	if err := a.repoAction(loc.Repository, teleservices.VerbRead); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if loc.Name == constants.OpsCenterCAPackage {
		if err := a.checker.CheckAccessToRule(a.repoContext(loc.Repository), teledefaults.Namespace, storage.KindRepository, storage.VerbReadSecrets, false); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	return a.packages.ReadPackage(loc)
}

// ReadPackageEnvelope returns package envelope
func (a *ACLService) ReadPackageEnvelope(loc loc.Locator) (*PackageEnvelope, error) {
	if err := a.repoAction(loc.Repository, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.packages.ReadPackageEnvelope(loc)
}

const (
	// CollectionRepositories means access on all repositories that exist
	CollectionRepositories = "repositories"
	// CollectionRepository means access on a particular repository
	CollectionRepository = "repository"
)
