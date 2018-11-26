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

package process

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/transfer"
	telecfg "github.com/gravitational/teleport/lib/config"
	teledefaults "github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// importer lets us import initial state from gravity packages
type importer struct {
	backend       storage.Backend
	packages      pack.PackageService
	exportPackage *loc.Locator
	dir           string
	// FieldLogger is used for logging
	logrus.FieldLogger
}

func newImporter(dir string) (*importer, error) {
	if dir == "" {
		return nil, trace.BadParameter("missing directory with packages")
	}
	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path:     filepath.Join(dir, defaults.GravityDBFile),
		Readonly: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i := &importer{
		backend: backend,
		dir:     dir,
		FieldLogger: logrus.WithFields(logrus.Fields{
			trace.Component:    "importer",
			constants.FieldDir: dir,
		})}
	err = func() error {
		objects, err := fs.New(filepath.Join(dir, defaults.PackagesDir))
		if err != nil {
			return trace.Wrap(err)
		}

		i.packages, err = localpack.New(localpack.Config{
			Backend:     backend,
			UnpackedDir: filepath.Join(dir, defaults.PackagesDir, defaults.UnpackedDir),
			Objects:     objects,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// TODO(klizhentas) fix FindPackages insanity and remove foreach
		err = pack.ForeachPackage(i.packages, func(e pack.PackageEnvelope) error {
			i.Infof("Looking at %v.", e.Locator)
			if e.Locator.Name == constants.SiteExportPackage {
				i.exportPackage = &e.Locator
			}
			return nil
		})
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}()
	if err != nil {
		i.Close()
		return nil, trace.Wrap(err)
	}
	return i, nil
}

// Close releases resources, e.g. locked database
func (i *importer) Close() error {
	i.Debug("Closing backend.")
	return i.backend.Close()
}

// getMasterTeleportConfig extracts configuration from teleport package
func (i *importer) getMasterTeleportConfig() (*telecfg.FileConfig, error) {
	configPackage, err := pack.FindLatestPackageByName(i.packages,
		constants.TeleportMasterConfigPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.Infof("Using teleport master config from %v.", configPackage)

	_, reader, err := i.packages.ReadPackage(*configPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()

	vars, err := pack.ReadConfigPackage(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configString, ok := vars[teledefaults.ConfigEnvar]
	if !ok {
		return nil, trace.NotFound(
			"variable %q not found in config", teledefaults.ConfigEnvar)
	}

	fileConf, err := telecfg.ReadFromString(configString)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fileConf, nil
}

func (i *importer) importState(b storage.Backend, localPackages pack.PackageService) error {
	if i.exportPackage == nil {
		i.Debug("No export package found.")
		return nil
	}
	if err := i.importSite(b); err != nil {
		return trace.Wrap(err)
	}
	if err := i.importPackages(localPackages); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (i *importer) importPackages(localPackages pack.PackageService) error {
	i.Debug("Importing packages.")
	err := pack.ForeachPackage(i.packages, func(e pack.PackageEnvelope) error {
		// no need to import site export data - it's one shot init thing
		if e.Locator.Name == constants.SiteExportPackage {
			return nil
		}
		start := time.Now()
		env, reader, err := i.packages.ReadPackage(e.Locator)
		if err != nil {
			return trace.Wrap(err)
		}
		defer reader.Close()

		labels := env.RuntimeLabels
		delete(labels, pack.InstalledLabel)

		_, err = localPackages.UpsertPackage(e.Locator, reader,
			pack.WithLabels(labels),
			pack.WithHidden(env.Hidden),
			pack.WithManifest(env.Type, env.Manifest),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		i.Debugf("Imported %v in %v.", e.Locator, time.Since(start))
		return nil
	})
	return trace.Wrap(err)
}

// importSite imports site into backend b from the
// backend state represented as `site-export` package.
// The result of the import is the backend b initialized
// with the site id, account id and other artefacts
// created by opscenter during install
func (i *importer) importSite(b storage.Backend) error {
	i.Debug("Importing cluster data.")
	tempDir, err := ioutil.TempDir(i.dir, "site-export")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(tempDir)
	if i.exportPackage.IsEmpty() {
		return trace.NotFound("export package not found")
	}
	_, reader, err := i.packages.ReadPackage(*i.exportPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	f, err := ioutil.TempFile(tempDir, "db")
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := io.Copy(f, reader); err != nil {
		f.Close()
		return trace.Wrap(err)
	}
	if err := f.Close(); err != nil {
		return trace.Wrap(err)
	}
	err = transfer.ImportSite(f.Name(), b)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
