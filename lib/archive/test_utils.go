//go:build !release
// +build !release

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

package archive

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1" //nolint:revive,stylecheck // TODO: tests will be rewritten to use testify
)

// AssertArchiveHasFiles validates that filenames are in the archive r.
func AssertArchiveHasFiles(c *C, r io.ReadCloser, excludePatterns []string, filenames ...string) {
	items := make([]TestItem, 0, len(filenames))
	for _, name := range filenames {
		items = append(items, Filename(name))
	}
	AssertArchiveHasItems(c, r, excludePatterns, items...)
}

// FetchFiles fetches archive files as map
func FetchFiles(r io.ReadCloser, excludePatterns []string) (map[string]string, error) {
	defer r.Close()

	var excludes []*regexp.Regexp
	for _, pattern := range excludePatterns {
		p, err := regexp.Compile(pattern)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		excludes = append(excludes, p)
	}

	out := make(map[string]string)
	stream, err := archive.DecompressStream(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	archive := tar.NewReader(stream)
archiveLoop:
	for {
		hdr, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, exclude := range excludes {
			if exclude.MatchString(hdr.Name) {
				continue archiveLoop
			}
		}
		if hdr.Typeflag == tar.TypeReg {
			data, err := ioutil.ReadAll(archive)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out[hdr.Name] = string(data)
		}
	}
	return out, nil
}

// AssertArchiveHasItems validates that items are in the archive r.
func AssertArchiveHasItems(c *C, r io.ReadCloser, excludePatterns []string, items ...TestItem) {
	defer r.Close()

	stream, err := archive.DecompressStream(r)
	c.Assert(err, IsNil)

	var excludes []*regexp.Regexp
	for _, pattern := range excludePatterns {
		p, err := regexp.Compile(pattern)
		c.Assert(err, IsNil)
		excludes = append(excludes, p)
	}

	remove := removeItem(items...)
	archive := tar.NewReader(stream)
archiveLoop:
	for {
		hdr, err := archive.Next()
		if err == io.EOF {
			break
		}
		c.Assert(err, IsNil)
		for _, exclude := range excludes {
			if exclude.MatchString(hdr.Name) {
				continue archiveLoop
			}
		}
		item := remove(hdr.Name)
		c.Assert(item, Not(IsNil), Commentf("no item matched for %v", hdr))
		item.AssertItem(c, archive, hdr)
	}

	c.Assert(items, Not(HasLen), 0, Commentf("%v not found in archive", items))
}

// AssertDirHasFiles validates that directory dir has files.
func AssertDirHasFiles(c *C, dir string, files ...TestFile) {
	remove := removeFile(files...)
	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if dir == path {
			return nil
		}
		localPath, err := filepath.Rel(dir, path)
		c.Assert(err, IsNil)
		result := remove(localPath)
		c.Assert(result, Not(IsNil), Commentf("no file matched for path %q", path))
		result.AssertFile(c, path, fi)
		return nil
	})
	c.Assert(err, IsNil)
	c.Assert(files, Not(HasLen), 0, Commentf("files %v not found in directory %q", files, dir))
}

func (r Filename) SameName(name string) bool {
	return string(r) == name || string(r)+"/" == name
}

func (r Filename) AssertItem(*C, *tar.Reader, *tar.Header) {}

type Filename string

type NameMatcher interface {
	SameName(name string) bool
}

type TestItem interface {
	NameMatcher
	AssertItem(c *C, tarball *tar.Reader, hdr *tar.Header)
}

type TestFile interface {
	NameMatcher
	AssertFile(c *C, path string, fi os.FileInfo)
}

func removeItem(items ...TestItem) func(string) TestItem {
	return func(name string) (result TestItem) {
		for i, item := range items {
			if item.SameName(name) {
				result = items[i]
				items = append(items[:i], items[i+1:]...)
				return result
			}
		}
		return nil
	}
}

func removeFile(files ...TestFile) func(string) TestFile {
	return func(name string) (result TestFile) {
		for i, file := range files {
			if file.SameName(name) {
				result = files[i]
				files = append(files[:i], files[i+1:]...)
				return result
			}
		}
		return nil
	}
}
