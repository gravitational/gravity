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

package hub

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/dustin/go-humanize"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// Hub defines an interface for the hub that stores Telekube application installers
//
// The current hub implementation is backed by S3 and all application installers
// are stored in the bucket of the following structure:
//
// hub.gravitational.io
// ∟ gravity
//   ∟ oss
//     ∟ index.yaml
//     ∟ app
//       ∟ telekube
//         ∟ 5.2.0
//           ∟ linux
//             ∟ x86_64
//               ∟ telekube-5.2.0-linux-x86_64.tar
//               ∟ telekube-5.2.0-linux-x86_64.tar.sha256
//         ∟ latest: // same as versioned sub-bucket
//         ∟ stable: // same as versioned sub-bucket
//
// The index file, index.yaml, provides information about installers stored
// in the bucket and is updated every time a new version is published. The
// index file format is the same as Helm's chart repository index file.
type Hub interface {
	// List returns a list of applications in the hub
	List(withPrereleases bool) ([]App, error)
	// Downloads downloads the specified application installer into provided file
	Download(*os.File, loc.Locator) error
	// Get returns application installer tarball of the specified version
	Get(loc.Locator) (io.ReadCloser, error)
	// GetLatestVersion returns latest version of the specified application
	GetLatestVersion(name string) (string, error)
}

// App represents a single application item in the hub
type App struct {
	// Name is the application name
	Name string `json:"name"`
	// Version is the application version
	Version string `json:"version"`
	// Created is the application creation timestamp
	Created time.Time `json:"created"`
	// SizeBytes is the application size in bytes
	SizeBytes int64 `json:"sizeBytes"`
	// Description is the image description
	Description string `json:"description"`
	// Type is the image type, for example application or cluster
	Type string `json:"type"`
}

// s3Hub is the S3-backed hub implementation
type s3Hub struct {
	// Config is the hub configuration
	Config
	// downloader is the S3 download manager
	downloader *s3manager.Downloader
}

// Config is the S3-backed hub configuration
type Config struct {
	// Bucket is the S3 bucket name
	Bucket string
	// Prefix is the S3 path prefix
	Prefix string
	// Region is the S3 region
	Region string
	// FieldLogger is used for logging
	logrus.FieldLogger
	// S3 is optional S3 API client
	S3 s3iface.S3API
}

// CheckAndSetDefaults validates config and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Bucket == "" {
		c.Bucket = defaults.HubBucket
	}
	if c.Prefix == "" {
		c.Prefix = defaults.HubTelekubePrefix
	}
	if c.Region == "" {
		c.Region = defaults.AWSRegion
	}
	if c.FieldLogger == nil {
		c.FieldLogger = logrus.WithField(trace.Component, "s3hub")
	}
	if c.S3 == nil {
		session, err := session.NewSession(&aws.Config{
			Region:      aws.String(c.Region),
			Credentials: credentials.AnonymousCredentials,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		c.S3 = s3.New(session)
	}
	return nil
}

// New returns a new S3-backed hub instance
func New(config Config) (*s3Hub, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &s3Hub{
		Config:     config,
		downloader: s3manager.NewDownloaderWithClient(config.S3),
	}, nil
}

// List returns a list of applications in the hub
func (h *s3Hub) List(withPrereleases bool) (items []App, err error) {
	indexFile, err := h.getIndexFile()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, entryVersions := range indexFile.Entries {
		for _, entry := range entryVersions {
			items = append(items, App{
				Name:        entry.Name,
				Version:     entry.Version,
				Created:     entry.Created,
				Description: strings.TrimSpace(entry.Description),
				Type:        entry.Annotations[constants.AnnotationKind],
			})
		}
	}
	return items, nil
}

// Downloads downloads the specified application installer into provided file
func (h *s3Hub) Download(f *os.File, locator loc.Locator) (err error) {
	version := locator.Version
	// in case the provided version is a special 'latest' or 'stable' label,
	// we need to look into respective bucket to find out the actual version
	switch version {
	case loc.LatestVersion:
		locator.Version, err = h.GetLatestVersion(locator.Name)
	case loc.StableVersion:
		locator.Version, err = h.getStableVersion(locator.Name)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	h.Infof("Downloading: %v.", h.appPath(locator.Name, locator.Version))
	n, err := h.downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(h.appPath(locator.Name, locator.Version)),
	})
	if err != nil {
		err := utils.ConvertS3Error(err)
		if trace.IsNotFound(err) {
			return trace.NotFound("image %v:%v not found in %v, use 'tele ls -a' to see available images",
				locator.Name, locator.Version, h.Bucket)
		}
		return trace.Wrap(err)
	}
	h.Infof("Download complete: %v %v.", locator, humanize.Bytes(uint64(n)))
	if err := h.verifyChecksum(locator.Name, locator.Version, f.Name()); err != nil {
		return trace.Wrap(err, "failed to verify %v:%v checksum", locator.Name, locator.Version)
	}
	return nil
}

// Get returns application installer tarball of the specified version
func (h *s3Hub) Get(locator loc.Locator) (io.ReadCloser, error) {
	tarFile, err := ioutil.TempFile("", locator.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	readCloser := &utils.CleanupReadCloser{
		ReadCloser: tarFile,
		Cleanup: func() {
			if err := os.RemoveAll(tarFile.Name()); err != nil {
				h.Warnf("Failed to remove %v: %v.", tarFile.Name(), err)
			}
		},
	}
	err = h.Download(tarFile, locator)
	if err != nil {
		readCloser.Close()
		return nil, trace.Wrap(err)
	}
	return readCloser, nil
}

// getIndexFile returns the hub's index file.
func (h *s3Hub) getIndexFile() (*repo.IndexFile, error) {
	object, err := h.S3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(filepath.Join(h.Prefix, indexFileName)),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer object.Body.Close()
	bytes, err := ioutil.ReadAll(object.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var indexFile repo.IndexFile
	err = yaml.Unmarshal(bytes, &indexFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &indexFile, nil
}

// appsBucket returns sub-bucket where all applications are stored
func (h *s3Hub) appsBucket() string {
	return fmt.Sprintf("%v/app", h.Prefix)
}

// appBucket returns sub-bucket where the specified application is stored
func (h *s3Hub) appBucket(name, version string) string {
	return fmt.Sprintf("%v/%v/%v/linux/x86_64", h.appsBucket(), name, version)
}

// appBucketPath returns path to the specified application in the hub
func (h *s3Hub) appPath(name, version string) string {
	return fmt.Sprintf("%v/%v", h.appBucket(name, version), makeFilename(name, version))
}

// shaPath returns path to the checksum file of the specified application in the hub
func (h *s3Hub) shaPath(name, version string) string {
	return h.appPath(name, version) + ".sha256"
}

// GetLatestVersion returns the latest version of the specified application in the hub
func (h *s3Hub) GetLatestVersion(name string) (string, error) {
	indexFile, err := h.getIndexFile()
	if err != nil {
		return "", trace.Wrap(err)
	}
	versions := indexFile.Entries[name]
	if len(versions) == 0 {
		return "", trace.NotFound("image %q not found", name)
	}
	latestVersion, err := semver.NewVersion(versions[0].Version)
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, version := range versions {
		nextVersion, err := semver.NewVersion(version.Version)
		if err != nil {
			return "", trace.Wrap(err)
		}
		if latestVersion.LessThan(*nextVersion) {
			latestVersion = nextVersion
		}
	}
	return latestVersion.String(), nil
}

// getStableVersion returns the stable version of the specified application in the hub
func (h *s3Hub) getStableVersion(name string) (string, error) {
	indexFile, err := h.getIndexFile()
	if err != nil {
		return "", trace.Wrap(err)
	}
	versions, ok := indexFile.Entries[name]
	if !ok || len(versions) == 0 {
		return "", trace.NotFound("image %q not found", name)
	}
	var stableVersion *semver.Version
	for _, version := range versions {
		nextVersion, err := semver.NewVersion(version.Version)
		if err != nil {
			return "", trace.Wrap(err)
		}
		if nextVersion.PreRelease != "" {
			continue
		}
		if stableVersion == nil || stableVersion.LessThan(*nextVersion) {
			stableVersion = nextVersion
		}
	}
	if stableVersion == nil {
		return "", trace.NotFound("no stable version of image %q found", name)
	}
	return stableVersion.String(), nil
}

// verifyChecksum verifies the checksum of the downloaded installer file
func (h *s3Hub) verifyChecksum(name, version, path string) error {
	storedChecksum, err := h.getChecksum(name, version)
	if err != nil {
		return trace.Wrap(err)
	}
	file, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return trace.Wrap(err)
	}
	checksum := fmt.Sprintf("%x", hash.Sum(nil))
	if storedChecksum != checksum {
		return trace.BadParameter("checksum mismatch: stored %q, calculated %q",
			storedChecksum, checksum)
	}
	h.Infof("Checksum for %v:%v verified: %v.", name, version, checksum)
	return nil
}

// getChecksum fetches the sha256 checksum of the specified application from the hub
func (h *s3Hub) getChecksum(name, version string) (string, error) {
	object, err := h.S3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(h.shaPath(name, version)),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer object.Body.Close()
	// even though the file should only contain the checksum, read in only
	// first 64 bytes (checksum block size) to be on a safe side
	checksum := make([]byte, sha256.BlockSize)
	n, err := bufio.NewReader(object.Body).Read(checksum)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if n != sha256.BlockSize {
		return "", trace.BadParameter("expected %v bytes but read only %v",
			sha256.BlockSize, n)
	}
	return string(checksum), nil
}

// makeFilename returns the name of the file under which the application
// specified by the provided locator is stored in the hub
func makeFilename(name, version string) string {
	return fmt.Sprintf("%v-%v-linux-x86_64.tar", name, version)
}

const (
	// indexFileName is the repository index file name.
	indexFileName = "index.yaml"
)
