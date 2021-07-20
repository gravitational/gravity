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

package fs

import (
	"crypto/sha512"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/systeminfo"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New creates a new instance of the local fs blob service
// rooted as the given path
func New(root string) (blob.Objects, error) {
	return NewWithConfig(Config{Path: root})
}

// NewWithConfig creates a new instance of the local fs blob service with the specified configuration
func NewWithConfig(config Config) (blob.Objects, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	o := &objects{config: config}
	for _, d := range []string{config.tempDir(), config.blobDir()} {
		if err := os.MkdirAll(d, defaults.SharedDirMask); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}
	return o, nil
}

// Config defines the blob service configuration
type Config struct {
	// Path specifies the directory for blobs
	Path string
	// User optionally specifies the user context for file operations
	User *systeminfo.User
}

func (r *Config) checkAndSetDefaults() error {
	if r.Path == "" {
		return trace.BadParameter("missing Path parameter")
	}
	return nil
}

func (r Config) tempDir() string {
	return filepath.Join(r.Path, "tmp")
}

func (r Config) blobDir() string {
	return filepath.Join(r.Path, "blobs")
}

type objects struct {
	config Config
}

// hashDir helps us to organize the blobs in the folder -
// instead of putting all blobs in one folder, we
// will put them in 4096 folders, groping by first 3 strings
// of the sha512 hash - this will allow to scale in cases
// when there are too many files in one directory
func (o *objects) hashDir(h string) string {
	return filepath.Join(o.config.blobDir(), h[0:3])
}

func (o *objects) Close() error {
	return nil
}

// GetBLOBs returns a list of BLOBs in the storage
func (o *objects) GetBLOBs() ([]string, error) {
	var out []string
	blobDir := o.config.blobDir()
	err := filepath.Walk(blobDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warningf("error while traversing %v: %v", blobDir, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		_, name := filepath.Split(info.Name())
		out = append(out, name)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Strings(out)
	return out, nil
}

// WriteBLOB writes object to the storage, returns object envelope
func (o *objects) WriteBLOB(data io.Reader) (*blob.Envelope, error) {
	// step1 : write data and compute it's hash to the temporary file,
	// then move it to the proper location based on it's hash
	f, err := ioutil.TempFile(o.config.tempDir(), "blob")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	// if true, sets proper directory/file ownership.
	hasUser := o.config.User != nil && os.Geteuid() != o.config.User.UID
	hasher := sha512.New()
	w := io.MultiWriter(f, hasher)
	size, err := io.Copy(w, data)
	if err != nil {
		defer os.Remove(f.Name())
		return nil, trace.Wrap(err)
	}
	if err := f.Close(); err != nil {
		defer os.Remove(f.Name())
		return nil, trace.Wrap(err)
	}
	// step2 : now once we computed the hash, move the file to the
	// right place in the folder structure
	hash := fmt.Sprintf("%x", hasher.Sum(nil)[:sha512.Size/2])
	targetDir := o.hashDir(hash)
	if err := os.MkdirAll(targetDir, defaults.SharedDirMask); err != nil {
		// This will fail as expected if the command is not run as root or
		// under a different user context
		defer os.Remove(f.Name())
		return nil, trace.ConvertSystemError(err)
	}
	if hasUser {
		if err := os.Chown(targetDir, o.config.User.UID, o.config.User.GID); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	// now place it to the right place in the filesystem
	targetPath := filepath.Join(targetDir, hash)
	if err := os.Rename(f.Name(), targetPath); err != nil {
		defer os.Remove(f.Name())
		return nil, trace.Wrap(err)
	}
	if hasUser {
		// This will fail as expected if the command is not run as root or
		// under a different user context
		if err := os.Chown(targetPath, o.config.User.UID, o.config.User.GID); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		if err2 := os.Remove(targetPath); err2 != nil {
			log.WithError(err).Errorf("Failed to remove %v.", targetPath)
		}
		return nil, trace.ConvertSystemError(err)
	}
	return &blob.Envelope{
		SizeBytes: size,
		SHA512:    hash,
		Modified:  fileInfo.ModTime().UTC(),
	}, nil
}

// GetBLOBEnvelope returns file information identified by hash
func (o *objects) GetBLOBEnvelope(hash string) (*blob.Envelope, error) {
	targetPath := filepath.Join(o.hashDir(hash), hash)
	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return &blob.Envelope{
		SizeBytes: fileInfo.Size(),
		SHA512:    hash,
		Modified:  fileInfo.ModTime().UTC(),
	}, nil
}

// OpenBLOB opens file identified by hash and returns reader
func (o *objects) OpenBLOB(hash string) (blob.ReadSeekCloser, error) {
	f, err := os.Open(filepath.Join(o.hashDir(hash), hash))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

// DeleteBLOB deletes BLOB from the storage
func (o *objects) DeleteBLOB(hash string) error {
	err := os.Remove(filepath.Join(o.hashDir(hash), hash))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}
