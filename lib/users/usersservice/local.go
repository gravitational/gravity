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

package usersservice

import (
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"gopkg.in/yaml.v2"
)

// NewLocalKeyStore returns new user-local key storage
func NewLocalKeyStore(path string) (*users.KeyStore, error) {
	path, err := utils.GetLocalPath(path, defaults.LocalConfigDir, defaults.LocalConfigFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return users.NewCredsService(users.CredsConfig{
		Backend: &LocalLogins{path: path, Clock: clockwork.NewRealClock()},
	})
}

// LocalLogins store local logins with remote ops centers
type LocalLogins struct {
	path string
	clockwork.Clock
}

type loginsFile struct {
	CurrentOpsCenter string               `yaml:"current"`
	OpsCenters       []storage.LoginEntry `yaml:"opscenters"`
}

func (l *LocalLogins) GetCurrentOpsCenter() string {
	f, err := l.readFile()
	if err != nil {
		return ""
	}
	return f.CurrentOpsCenter
}

func (l *LocalLogins) SetCurrentOpsCenter(o string) error {
	f, err := l.readFile()
	if err != nil {
		return err
	}
	f.CurrentOpsCenter = o
	return l.writeFile(f)
}

func (l *LocalLogins) UpsertLoginEntry(e storage.LoginEntry) (*storage.LoginEntry, error) {
	f, err := l.readFile()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var found bool
	for i := range f.OpsCenters {
		if f.OpsCenters[i].OpsCenterURL == e.OpsCenterURL {
			f.OpsCenters[i] = e
			found = true
			break
		}
	}
	if !found {
		f.OpsCenters = append(f.OpsCenters, e)
	}
	if err = l.writeFile(f); err != nil {
		return nil, trace.Wrap(err)
	}
	return &e, nil
}

func (l *LocalLogins) GetLoginEntries() ([]storage.LoginEntry, error) {
	f, err := l.readFile()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return f.OpsCenters, nil
}

func (l *LocalLogins) GetLoginEntry(opsCenterURL string) (*storage.LoginEntry, error) {
	f, err := l.readFile()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := range f.OpsCenters {
		if f.OpsCenters[i].OpsCenterURL == opsCenterURL {
			return &f.OpsCenters[i], nil
		}
	}
	return nil, trace.NotFound("opscenter %v not found", opsCenterURL)
}

func (l *LocalLogins) DeleteLoginEntry(opsCenterURL string) error {
	f, err := l.readFile()
	if err != nil {
		return trace.Wrap(err)
	}
	for i := range f.OpsCenters {
		if f.OpsCenters[i].OpsCenterURL == opsCenterURL {
			f.OpsCenters = append(f.OpsCenters[:i], f.OpsCenters[i+1:]...)
			return l.writeFile(f)
		}
	}
	return trace.NotFound("%v not found", opsCenterURL)
}

func (l *LocalLogins) readFile() (*loginsFile, error) {
	data, err := utils.ReadPath(l.path)
	if err != nil {
		if trace.IsNotFound(err) {
			return &loginsFile{}, nil
		}
		return nil, trace.Wrap(err)
	}
	var f loginsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.LoginEntry
	for i := range f.OpsCenters {
		utils.UTC(&f.OpsCenters[i].Expires)
		if f.OpsCenters[i].Expires.IsZero() || !l.Now().After(f.OpsCenters[i].Expires) {
			out = append(out, f.OpsCenters[i])
		}
	}
	f.OpsCenters = out
	return &f, nil
}

func (l *LocalLogins) writeFile(f *loginsFile) error {
	if _, err := utils.EnsureLocalPath(l.path, "", ""); err != nil {
		return trace.Wrap(err)
	}
	out, err := yaml.Marshal(f)
	if err != nil {
		return trace.Wrap(err)
	}
	return utils.WritePath(l.path, out, defaults.PrivateFileMask)
}
