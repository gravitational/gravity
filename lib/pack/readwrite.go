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
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
)

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

func WritePackage(m Manifest, w io.Writer, files []PackageFile) error {
	wc, err := archive.CompressStream(&nopWriteCloser{w}, archive.Gzip)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(w)
	defer wc.Close()

	mb, err := m.EncodeJSON()
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name: "orbit.manifest.json",
		Size: int64(len(mb)),
		Mode: 0444,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return trace.Wrap(err, "error writing manifest header")
	}
	if _, err := tw.Write(mb); err != nil {
		return trace.Wrap(err, "error writing manifest body")
	}

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.Path,
			Size: int64(len(f.Contents)),
			Mode: 0644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return trace.Wrap(err, "error writing tar header")
		}
		if _, err := tw.Write(f.Contents); err != nil {
			return trace.Wrap(err, "error writing file body")
		}
	}

	if err := tw.Close(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func ReadPackage(r io.Reader) (*Manifest, []PackageFile, error) {
	rc, err := archive.DecompressStream(r)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer rc.Close()
	tr := tar.NewReader(rc)

	p := []PackageFile{}
	var m *Manifest

	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, trace.Wrap(err)
		}
		if h.FileInfo().IsDir() {
			continue
		}

		h.Name = filepath.Clean(h.Name)
		if filepath.Base(h.Name) == "orbit.manifest.json" {
			m, err = ParseManifestJSON(tr)
			if err != nil {
				return nil, nil, err
			}
		} else {
			bytes, err := ioutil.ReadAll(tr)
			if err != nil {
				return nil, nil, trace.Wrap(err, fmt.Sprintf("error reading %v", h.Name))
			}
			p = append(p, PackageFile{
				Path:     h.Name,
				Contents: bytes,
			})
		}
	}
	if m == nil {
		return nil, nil, trace.Errorf("manifest not found")
	}
	return m, p, nil
}

type PackageFile struct {
	Path     string
	Contents []byte
}
