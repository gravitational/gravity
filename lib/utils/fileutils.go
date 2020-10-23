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

package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// RecursiveGlob recursively walks the dir and returns the list of files
// matching the specified patterns.
func RecursiveGlob(dir string, patterns []string, handler func(match string) error) error {
	err := filepath.Walk(dir, func(filePath string, fi os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if !fi.IsDir() {
			for _, pattern := range patterns {
				matched, _ := filepath.Match(pattern, filepath.Base(filePath))
				if matched {
					if err = handler(filePath); err != nil {
						return trace.Wrap(err)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NormalizePath normalises path, evaluating symlinks and converting local
// paths to absolute
func NormalizePath(path string) (string, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	abs, err := filepath.EvalSymlinks(s)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return abs, nil
}

// MkdirAll creates directory and subdirectories
func MkdirAll(targetDirectory string, mode os.FileMode) error {
	err := os.MkdirAll(targetDirectory, mode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// WritePath writes file to given path
func WritePath(path string, data []byte, perm os.FileMode) error {
	err := ioutil.WriteFile(path, data, perm)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// ReadPath reads file at given path
func ReadPath(path string) ([]byte, error) {
	abs, err := NormalizePath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bytes, err := ioutil.ReadFile(abs)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return bytes, nil
}

// ReaderForPath returns a reader for file at given path
func ReaderForPath(path string) (io.ReadCloser, error) {
	abs, err := NormalizePath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

// StatDir stats directory, returns error if file exists, but not a directory
func StatDir(path string) (os.FileInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if !fi.IsDir() {
		return nil, trace.BadParameter("%v is not a directory", path)
	}
	return fi, nil
}

// StatFile determines if the specified path refers to a file.
// Returns file information on success.
// If path refers to a directory, an error is returned
func StatFile(path string) (os.FileInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if fi.IsDir() {
		return nil, trace.BadParameter("%v is not a file", path)
	}
	return fi, nil
}

// IsFile determines if path specifies a regular file
func IsFile(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}
	return !fi.IsDir() && fi.Mode().IsRegular(), nil
}

// IsDirectory determines if path specifies a directory
func IsDirectory(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}
	return fi.IsDir(), nil
}

// IsDirectoryEmpty returns true if the specified directory is empty
// The directory must exist or an error will be returned
func IsDirectoryEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}
	defer f.Close()
	if _, err = f.Readdirnames(1); err == io.EOF {
		return true, nil
	}
	return false, trace.ConvertSystemError(err)
}

// CopyDirContents copies all contents of the source directory to the destination
// directory
func CopyDirContents(srcDir, dstDir string) error {
	// create dest directory if it doesn't exist
	err := os.MkdirAll(dstDir, defaults.SharedDirMask)
	if err != nil {
		return trace.Wrap(err)
	}
	srcDir = filepath.Clean(srcDir)
	err = filepath.Walk(srcDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}

		// ignore root directory
		if path == srcDir {
			return nil
		}

		if fi.IsDir() {
			// create directory for the target file
			targetDir := filepath.Join(dstDir, strings.TrimPrefix(path, srcDir))
			err = os.MkdirAll(targetDir, defaults.SharedDirMask)
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			// copy sub-directories recursively
			return nil
		}

		relativePath := strings.TrimPrefix(filepath.Dir(path), srcDir)
		targetDir := filepath.Join(dstDir, relativePath)

		// copy file, preserve permissions
		toFileName := filepath.Join(targetDir, filepath.Base(fi.Name()))
		err = CopyFileWithPerms(toFileName, path, fi.Mode())
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	return trace.Wrap(err)
}

// EnsureLocalPath makes sure the path exists, or, if omitted results in the subpath in
// default gravity config directory, e.g.
//
// EnsureLocalPath("/custom/myconfig", ".gravity", "config") -> /custom/myconfig
// EnsureLocalPath("", ".gravity", "config") -> ${HOME}/.gravity/config
//
// It also makes sure that base dir exists
func EnsureLocalPath(customPath, defaultLocalDir, defaultLocalPath string) (string, error) {
	path, err := GetLocalPath(customPath, defaultLocalDir, defaultLocalPath)
	if err != nil {
		return "", trace.Wrap(err)
	}
	baseDir := filepath.Dir(path)
	_, err = StatDir(baseDir)
	if err != nil {
		if trace.IsNotFound(err) {
			if err := MkdirAll(baseDir, defaults.PrivateDirMask); err != nil {
				return "", trace.Wrap(err)
			}
		} else {
			return "", trace.Wrap(err)
		}
	}
	return path, nil
}

// GetLocalPath constructs path to the local gravity config file like described
// in the EnsureLocalPath above.
func GetLocalPath(customPath, defaultLocalDir, defaultLocalPath string) (string, error) {
	if customPath != "" {
		return customPath, nil
	}
	homeDir := os.Getenv(constants.EnvHome)
	if homeDir == "" {
		return "", trace.NotFound("no path provided and environment variable %v is not set", constants.EnvHome)
	}
	return filepath.Join(homeDir, defaultLocalDir, defaultLocalPath), nil
}

// CopyFile copies contents of src to dst atomically
// using SharedReadWriteMask as permissions.
func CopyFile(dst, src string) error {
	return CopyFileWithPerms(dst, src, defaults.SharedReadWriteMask)
}

// CopyReader copies contents of src to dst atomically
// using SharedReadWriteMask as permissions.
func CopyReader(dst string, src io.Reader) error {
	return CopyReaderWithPerms(dst, src, defaults.SharedReadWriteMask)
}

// CopyFileWithPerms copies the contents from src to dst atomically.
// Uses CopyReaderWithPerms for its implementation - see function documentation
// for details of operation
func CopyFileWithPerms(dst, src string, perm os.FileMode) error {
	return CopyFileWithOptions(dst, src, PermOption(perm))
}

// CopyFileWithOptions copies the contents from src to dst atomically.
// Applies specified options to the target path
func CopyFileWithOptions(dst, src string, options ...FileOption) error {
	in, err := os.Open(src)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer in.Close()
	return CopyReaderWithOptions(dst, in, options...)
}

// CopyExecutable copies the provided reader to the specified destination and
// sets executable permissions.
func CopyExecutable(dst string, src io.Reader) error {
	return CopyReaderWithPerms(dst, src, defaults.SharedExecutableMask)
}

// CopyReaderWithPerms copies the contents from src to dst atomically.
// If dst does not exist, CopyReaderWithPerms creates it with permissions perm.
// If the copy fails, CopyReaderWithPerms aborts and dst is preserved.
// Adopted with modifications from https://go-review.googlesource.com/#/c/1591/9/src/io/ioutil/ioutil.go
func CopyReaderWithPerms(dst string, src io.Reader, perm os.FileMode) error {
	return CopyReaderWithOptions(dst, src, PermOption(perm))
}

// CopyReaderWithOptions copies the contents from src to dst atomically.
// If dst does not exist, CopyReaderWithOptions creates it.
// Callers choose the options to apply on the resulting file with options
func CopyReaderWithOptions(dst string, src io.Reader, options ...FileOption) error {
	tmp, err := ioutil.TempFile(filepath.Dir(dst), "")
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	cleanup := func() {
		err := os.Remove(tmp.Name())
		if err != nil {
			log.WithError(err).Warnf("Failed to remove %q.", tmp.Name())
		}
	}

	_, err = io.Copy(tmp, src)
	if err != nil {
		tmp.Close()
		cleanup()
		return trace.ConvertSystemError(err)
	}
	if err = tmp.Close(); err != nil {
		cleanup()
		return trace.ConvertSystemError(err)
	}
	for _, option := range options {
		if err = option(tmp.Name()); err != nil {
			cleanup()
			return trace.ConvertSystemError(err)
		}
	}
	err = os.Rename(tmp.Name(), dst)
	if err != nil {
		cleanup()
		return trace.ConvertSystemError(err)
	}
	return nil
}

// FileOption defines a functional option to apply to specified path
type FileOption func(path string) error

// PermOption changes the file permissions on the specified file
// to perm
func PermOption(perm os.FileMode) FileOption {
	return func(path string) error {
		return os.Chmod(path, perm)
	}
}

// OwnerOption changes the owner on the specified file
// to (uid, gid)
func OwnerOption(uid, gid int) FileOption {
	return func(path string) error {
		return os.Chown(path, uid, gid)
	}
}

// CleanupReadCloser is an io.ReadCloser that tracks when the reading side is closed
// and then runs the configured cleanup callback.
type CleanupReadCloser struct {
	io.ReadCloser
	Cleanup func()
}

// Read delegates reading to the underlying io.Reader
// Implements io.Reader
func (r *CleanupReadCloser) Read(p []byte) (int, error) {
	return r.ReadCloser.Read(p)
}

// Close delegates to the underlying io.Reader and runs the specified Cleanup.
// Implements io.Closer
func (r *CleanupReadCloser) Close() (err error) {
	err = r.ReadCloser.Close()
	r.Cleanup()
	return trace.Wrap(err)
}

// WithTempDir creates a temporary directory and executes the specified function fn
// providing it with the name of the directory.
// After fn is finished, the directory is automatically removed.
func WithTempDir(fn func(dir string) error, prefix string) error {
	dir, err := ioutil.TempDir("", prefix)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer os.RemoveAll(dir)

	err = fn(dir)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// RemoveContents removes any children of dir.
// It removes everything it can but returns the first error
// it encounters. If the dir does not exist, RemoveContents
// returns nil.
func RemoveContents(dir string) error {
	fd, err := os.Open(dir)
	if err != nil {
		err = trace.ConvertSystemError(err)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	defer fd.Close()
	names, err := fd.Readdirnames(-1)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

// OpenFile opens the file at the provided path in a+ mode
func OpenFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, defaults.SharedReadMask)
}

// EnsureLineInFile makes sure the specified file contains provided line
func EnsureLineInFile(path, line string) error {
	file, err := OpenFile(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == strings.TrimSpace(line) {
			return trace.AlreadyExists("line %q already exists", line)
		}
	}
	if err := scanner.Err(); err != nil {
		return trace.Wrap(err)
	}
	writer := bufio.NewWriter(file)
	if _, err := writer.WriteString(fmt.Sprintf("\n%v", strings.TrimSpace(line))); err != nil {
		return trace.Wrap(err)
	}
	if err := writer.Flush(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Chown adjusts ownership of the specified directory and all its subdirectories
func Chown(path string, uid, gid int) error {
	out, err := exec.Command("chown", "-R", fmt.Sprintf("%v:%v", uid, gid), path).CombinedOutput()
	if err != nil {
		return trace.Wrap(err, "failed to chown %q to %v:%v: %s", path, uid, gid, out)
	}
	return nil
}

// CopyWithRetries copies the contents of the reader obtained with open to targetPath
// retrying on transient errors
func CopyWithRetries(ctx context.Context, targetPath string, open func() (io.ReadCloser, error), options ...FileOption) error {
	b := backoff.NewConstantBackOff(defaults.RetryInterval)
	err := RetryTransient(ctx, b, func() error {
		rc, err := open()
		if err != nil {
			return trace.Wrap(err)
		}
		defer rc.Close()

		err = CopyReaderWithOptions(targetPath, rc, options...)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}
