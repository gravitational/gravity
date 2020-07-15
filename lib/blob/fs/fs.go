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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func New(path string) (blob.Objects, error) {
	if path == "" {
		return nil, trace.BadParameter("missing Path parameter")
	}
	o := &objects{dir: path}
	for _, d := range []string{o.tempDir(), o.blobDir()} {
		if err := os.MkdirAll(d, defaults.SharedDirMask); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}
	return o, nil
}

type objects struct {
	dir string
}

func (o *objects) tempDir() string {
	return filepath.Join(o.dir, "tmp")
}

func (o *objects) blobDir() string {
	return filepath.Join(o.dir, "blobs")
}

// hashDir helps us to organize the blobs in the folder -
// instead of putting all blobs in one folder, we
// will put them in 4096 folders, groping by first 3 strings
// of the sha512 hash - this will allow to scale in cases
// when there are too many files in one directory
func (o *objects) hashDir(h string) string {
	return filepath.Join(o.blobDir(), h[0:3])
}

func (o *objects) Close() error {
	return nil
}

// GetBLOBs returns a list of BLOBs in the storage
func (o *objects) GetBLOBs() ([]string, error) {
	var out []string
	blobDir := o.blobDir()
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
	f, err := ioutil.TempFile(o.tempDir(), "blob")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

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
		defer os.Remove(f.Name())
		return nil, trace.ConvertSystemError(err)
	}
	// now place it to the right place in the filesystem
	targetPath := filepath.Join(targetDir, hash)
	if err := os.Rename(f.Name(), targetPath); err != nil {
		defer os.Remove(f.Name())
		return nil, trace.Wrap(err)
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
