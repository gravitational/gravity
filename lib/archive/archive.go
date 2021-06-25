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
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// DefaultOptions returns an options object with sensible defaults
func DefaultOptions() *dockerarchive.TarOptions {
	options := &dockerarchive.TarOptions{NoLchown: true}
	return options
}

// CompressDirectory compresses the directory given with dir, using writer as a sink
// for the archive
func CompressDirectory(dir string, writer io.Writer, items ...*Item) error {
	archive := NewTarAppender(writer)
	defer archive.Close()

	if err := archive.Add(items...); err != nil {
		return trace.Wrap(err, "failed to write tarball: %v", err.Error())
	}
	if err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		localPath, err := filepath.Rel(dir, path)
		if err != nil {
			return trace.Wrap(err)
		}
		if localPath == "." {
			// Skip current directory item
			return nil
		}
		item, err := ItemFromFile(localPath, path, fi)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(archive.Add(item))
	}); err != nil {
		return trace.Wrap(err, "failed to compress directory %q", dir)
	}
	return nil
}

// Unpack unpacks the specified tarball to a temporary directory and returns
// the directory where it was unpacked
func Unpack(path string) (unpackedDir string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	defer file.Close()
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	err = Extract(file, tmp)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return tmp, nil
}

// Extract extracts the contents of the specified tarball under dir.
// The resulting files and directories are created using the current user context.
func Extract(r io.Reader, dir string) error {
	tarball := tar.NewReader(r)
	for {
		header, err := tarball.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return trace.Wrap(err)
		}

		// Security: ensure tar doesn't refer to file paths outside the directory
		err = SanitizeTarPath(header, dir)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := extractFile(tarball, header, dir, header.Name); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ExtractWithPrefix extracts the contents of the specified tarball under directory dir.
// Only the files/directories found in tarDirPrefix in the tarball matching patterns are extracted.
// Note, files from tarDir are written directly to dir omitting tarDir.
// The resulting files and directories are created using the current user context.
func ExtractWithPrefix(r io.Reader, dir, tarDirPrefix string) error {
	err := TarGlobWithPrefix(tar.NewReader(r), tarDirPrefix, func(match *tar.Header, r *tar.Reader) error {
		// Security: ensure tar doesn't refer to file paths outside the directory
		// Note, it validates the path as if the file/directory described by match
		// would have been extracted as-is while it is, in fact, extracted without the
		// top-level directory. It should not affect the security check though
		err := SanitizeTarPath(match, dir)
		if err != nil {
			return trace.Wrap(err)
		}
		relpath, err := filepath.Rel(tarDirPrefix, match.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := extractFile(r, match, dir, relpath); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	return trace.Wrap(err)
}

// HasFile returns nil if the specified tarball contains specified file
func HasFile(tarballPath, filename string) error {
	file, err := os.Open(tarballPath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer file.Close()
	var hasFile bool
	err = TarGlob(tar.NewReader(file), ".", []string{filename},
		func(match string, file io.Reader) error {
			hasFile = true
			return ErrAbort
		})
	if err != nil {
		if trace.Unwrap(err) == tar.ErrHeader {
			return trace.BadParameter("file %v does not appear to be a valid tarball",
				tarballPath)
		}
		return trace.Wrap(err)
	}
	if !hasFile {
		return trace.NotFound("tarball %v does not contain file %v",
			tarballPath, filename)
	}
	return nil
}

// TarGlob iterates the contents of the specified tarball and invokes handler
// for each file matching the list of specified patterns.
// If the handler returns a special Abort error, iteration will be aborted without errors.
func TarGlob(source *tar.Reader, dir string, patterns []string, handler func(match string, file io.Reader) error) (err error) {
	for {
		var hdr *tar.Header
		hdr, err = source.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return trace.Wrap(err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		for _, pattern := range patterns {
			relpath, err := filepath.Rel(dir, hdr.Name)
			if err == nil {
				matched, _ := filepath.Match(pattern, filepath.Base(relpath))
				if matched {
					if err = handler(relpath, source); err != nil {
						if trace.Unwrap(err) == ErrAbort {
							return nil
						}
						return trace.Wrap(err)
					}
				}
			}
		}
	}
	return nil
}

// TarGlobWithPrefix iterates the contents of the specified tarball and invokes handler
// for each file in the directory with the specified prefix (and all its sub-directories).
// If the handler returns a special Abort error, iteration will be aborted without errors.
func TarGlobWithPrefix(source *tar.Reader, prefix string, handler TarGlobHandler) (err error) {
	for {
		var hdr *tar.Header
		hdr, err = source.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return trace.Wrap(err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		path := filepath.Clean(hdr.Name)
		if strings.HasPrefix(path, prefix) {
			if err = handler(hdr, source); err != nil {
				if trace.Unwrap(err) == ErrAbort {
					return nil
				}
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// TarGlobHandler defines a handler for the match when iterating files in the tarball r.
type TarGlobHandler func(match *tar.Header, r *tar.Reader) error

// TarAppender wraps a tar writer and can append items to it
type TarAppender struct {
	tw *tar.Writer
}

// NewTarAppender creates a new tar appender writing to w
func NewTarAppender(w io.Writer) *TarAppender {
	return &TarAppender{tar.NewWriter(w)}
}

// Add adds the specified items to the underlined archive
func (r *TarAppender) Add(items ...*Item) (err error) {
	defer func() {
		for _, item := range items {
			if item.Data != nil {
				item.Data.Close()
			}
		}
	}()
	for _, item := range items {
		if item.ModTime.IsZero() {
			item.ModTime = time.Now()
		}
		if err = r.tw.WriteHeader(&item.Header); err != nil {
			return trace.Wrap(err)
		}
		if item.Data == nil {
			continue
		}
		if _, err = io.Copy(r.tw, item.Data); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Close closes the underlined archive
func (r *TarAppender) Close() error {
	return r.tw.Close()
}

// ItemFromString creates an Item from given string
func ItemFromString(path, value string) *Item {
	return ItemFromStringMode(path, value, defaults.SharedExecutableMask)
}

// DirItem creates a new virtual directory item
func DirItem(path string) *Item {
	return &Item{
		Header: tar.Header{
			Name:     path + "/",
			Typeflag: tar.TypeDir,
			Mode:     defaults.SharedDirMask,
			Uid:      defaults.ArchiveUID,
			Gid:      defaults.ArchiveGID,
		},
	}
}

// ItemFromStringMode creates a new Item from a given string with the provided permissions
func ItemFromStringMode(path, value string, mode int64) *Item {
	return ItemFromStream(path, ioutil.NopCloser(strings.NewReader(value)), int64(len(value)), mode)
}

// ItemFromStream creates an Item from given io.ReadCloser
func ItemFromStream(path string, rc io.ReadCloser, size, mode int64) *Item {
	return &Item{
		Header: tar.Header{
			Name: path,
			Size: size,
			Mode: mode,
			Uid:  defaults.ArchiveUID,
			Gid:  defaults.ArchiveGID,
		},
		Data: rc,
	}
}

// ItemFromFile creates an Item from the specified file
func ItemFromFile(localPath, path string, fi os.FileInfo) (*Item, error) {
	fiHeader, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &Item{
		Header: *fiHeader,
	}
	item.Name = localPath
	if !fi.IsDir() {
		item.Data, err = os.Open(path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return item, nil
}

// Item defines a unit of compression
type Item struct {
	tar.Header
	// Data is the data payload
	Data io.ReadCloser
}

// CreateMemArchive creates in-memory archive from archive items
func CreateMemArchive(items []*Item) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	archive := NewTarAppender(buf)

	if err := archive.Add(items...); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := archive.Close(); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf, nil
}

// MustCreateMemArchive creates in-memory archive from archive items
func MustCreateMemArchive(items []*Item) *bytes.Buffer {
	r, err := CreateMemArchive(items)
	if err != nil {
		panic(err)
	}
	return r
}

// ErrAbort is a special error value used to abort an iteration
var ErrAbort = errors.New("abort iteration")

// extractFile extracts a single file or directory from tarball into dir.
// Uses header to determine the type of item to create
// Based on https://github.com/mholt/archiver
func extractFile(tarball *tar.Reader, header *tar.Header, dir, path string) error {
	targetPath := filepath.Join(dir, path)
	switch header.Typeflag {
	case tar.TypeDir:
		return withDir(targetPath, nil)
	case tar.TypeBlock, tar.TypeChar, tar.TypeReg, tar.TypeRegA, tar.TypeFifo:
		return writeFile(targetPath, tarball, header.FileInfo().Mode())
	case tar.TypeLink:
		//nolint:gosec // the linkname had been sanitized with SanitizeTarPath
		return writeHardLink(targetPath, filepath.Join(dir, header.Linkname))
	case tar.TypeSymlink:
		return writeSymbolicLink(targetPath, header.Linkname)
	default:
		log.Warnf("unsupported type flag %v for %v", header.Typeflag, header.Name)
	}
	return nil
}

// SanitizeTarPath checks that the tar header paths resolve to a subdirectory path, and don't contain file paths or
// links that could escape the tar file (e.g. ../../etc/passwrd)
func SanitizeTarPath(header *tar.Header, dir string) error {
	// Security: sanitize that all tar paths resolve to within the destination directory
	//nolint:gosec
	destPath := filepath.Join(dir, header.Name)
	if !strings.HasPrefix(destPath, filepath.Clean(dir)+string(os.PathSeparator)) {
		return trace.BadParameter("%s: illegal file path", header.Name).AddField("prefix", dir)
	}
	// Security: Ensure link destinations resolve to within the destination directory
	if header.Linkname != "" {
		if filepath.IsAbs(header.Linkname) {
			if !strings.HasPrefix(filepath.Clean(header.Linkname), filepath.Clean(dir)+string(os.PathSeparator)) {
				return trace.BadParameter("%s: illegal link path", header.Linkname).AddField("prefix", dir)
			}
		} else {
			// relative paths are relative to the filename after extraction to a directory
			//nolint:gosec
			linkPath := filepath.Join(dir, filepath.Dir(header.Name), header.Linkname)
			if !strings.HasPrefix(linkPath, filepath.Clean(dir)+string(os.PathSeparator)) {
				return trace.BadParameter("%s: illegal link path", header.Linkname).AddField("prefix", dir)
			}
		}
	}
	return nil
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	err := withDir(path, func() error {
		out, err := os.Create(path)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		defer out.Close()

		err = out.Chmod(mode)
		if err != nil {
			return trace.ConvertSystemError(err)
		}

		_, err = io.Copy(out, r)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func writeSymbolicLink(path string, target string) error {
	err := withDir(path, func() error {
		err := os.Symlink(target, path)
		return trace.ConvertSystemError(err)
	})
	return trace.Wrap(err)
}

func writeHardLink(path string, target string) error {
	err := withDir(path, func() error {
		err := os.Link(target, path)
		return trace.ConvertSystemError(err)
	})
	return trace.Wrap(err)
}

func withDir(path string, fn func() error) error {
	err := os.MkdirAll(filepath.Dir(path), defaults.SharedDirMask)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	if fn == nil {
		return nil
	}
	err = fn()
	return trace.Wrap(err)
}
