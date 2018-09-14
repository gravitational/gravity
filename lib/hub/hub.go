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
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
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
// ∟ telekube
//   ∟ oss
//     ∟ app
//       ∟ telekube
//         ∟ 5.2.0
//           ∟ linux
//             ∟ x86_64
//               ∟ telekube-5.2.0-linux-x86_64.tar
//               ∟ telekube-5.2.0-linux-x86_64.tar.sha256
//         ∟ latest: // same as versioned sub-bucket
//         ∟ stable: // same as versioned sub-bucket
type Hub interface {
	// List returns a list of applications in the hub
	List() ([]App, error)
	// Downloads downloads the specified application installer into provided file
	Download(*os.File, loc.Locator, utils.Progress) error
	// Get returns application installer tarball of the specified version
	Get(loc.Locator) (io.ReadCloser, error)
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
}

// s3Hub is the S3-backed hub implementation
type s3Hub struct {
	// Config is the hub configuration
	Config
	// downloader is the S3 download manager
	downloader *s3manager.Downloader
	// appRegex is used to parse app information from S3 key
	appRegex *regexp.Regexp
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
			Region: aws.String(c.Region),
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
	appRegex, err := regexp.Compile(fmt.Sprintf(appPathRe, config.Prefix))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &s3Hub{
		Config:     config,
		downloader: s3manager.NewDownloaderWithClient(config.S3),
		appRegex:   appRegex,
	}, nil
}

// List returns a list of applications in the hub
func (h *s3Hub) List() ([]App, error) {
	objects, err := h.S3.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:  aws.String(h.Bucket),
		Prefix:  aws.String(h.appsBucket()),
		MaxKeys: aws.Int64(1000),
	})
	if err != nil {
		return nil, utils.ConvertS3Error(err)
	}
	var items []App
	for _, object := range objects.Contents {
		key := aws.StringValue(object.Key)
		if !strings.HasSuffix(key, constants.TarExtension) {
			continue
		}
		match := h.appRegex.FindStringSubmatch(key)
		if match == nil || len(match) < 3 {
			h.Warnf("Failed to parse the key: %q.", key)
			continue
		}
		switch match[2] {
		case constants.LatestVersion, constants.StableVersion:
			continue
		}
		items = append(items, App{
			Name:      match[1],
			Version:   match[2],
			Created:   aws.TimeValue(object.LastModified),
			SizeBytes: aws.Int64Value(object.Size),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Created.Before(items[j].Created)
	})
	return items, nil
}

// Downloads downloads the specified application installer into provided file
func (h *s3Hub) Download(f *os.File, locator loc.Locator, progress utils.Progress) (err error) {
	version := locator.Version
	// in case the provided version is a special 'latest' or 'stable' label,
	// we need to look into respective bucket to find out the actual version
	switch version {
	case loc.LatestVersion:
		locator.Version, err = h.getLatestVersion(locator.Name)
	case loc.StableVersion:
		locator.Version, err = h.getStableVersion(locator.Name)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	progress.NextStep(fmt.Sprintf("Downloading %v:%v", locator.Name, locator.Version))
	h.Infof("Downloading: %v.", h.appPath(locator.Name, locator.Version))
	n, err := h.downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(h.appPath(locator.Name, locator.Version)),
	})
	if err != nil {
		err := utils.ConvertS3Error(err)
		if trace.IsNotFound(err) {
			return trace.NotFound("application %v:%v not found in %v, use 'tele ls' to see available applications",
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
	err = h.Download(tarFile, locator, utils.NewNopProgress())
	if err != nil {
		readCloser.Close()
		return nil, trace.Wrap(err)
	}
	return readCloser, nil
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

// getLatestVersion returns the latest version of the specified application in the hub
func (h *s3Hub) getLatestVersion(name string) (string, error) {
	filename, err := h.getFilename(name, constants.LatestVersion)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return parseVersion(name, filename)
}

// getStableVersion returns the stable version of the specified application in the hub
func (h *s3Hub) getStableVersion(name string) (string, error) {
	filename, err := h.getFilename(name, constants.StableVersion)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return parseVersion(name, filename)
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

// getFilename returns the name of the file of the application installer tarball
// which is stored in the hub under specified name / version sub-bucket
func (h *s3Hub) getFilename(name, version string) (filename string, err error) {
	objects, err := h.S3.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(h.Bucket),
		Prefix: aws.String(h.appBucket(name, version)),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	// in this sub-bucket there should be the installer tarball and
	// its checksum, so let's find the installer filename
	for _, object := range objects.Contents {
		key := aws.StringValue(object.Key)
		if strings.HasSuffix(key, constants.TarExtension) {
			// key contains full path to the object so we need to trim
			// the prefix to get only the filename
			filename = strings.TrimPrefix(key, h.appBucket(name, version)+"/")
			break
		}
	}
	if filename == "" {
		return "", trace.NotFound("application %v:%v not found", name, version)
	}
	return filename, nil
}

// makeFilename returns the name of the file under which the application
// specified by the provided locator is stored in the hub
func makeFilename(name, version string) string {
	return fmt.Sprintf("%v-%v-linux-x86_64.tar", name, version)
}

// parseVersion extracts version from the provided filename for the specified
// application
func parseVersion(name, filename string) (string, error) {
	re, err := regexp.Compile(fmt.Sprintf(appFilenameRe, name))
	if err != nil {
		return "", trace.Wrap(err)
	}
	match := re.FindStringSubmatch(filename)
	if match == nil || len(match) < 2 {
		return "", trace.BadParameter("failed to parse version from %v for %q",
			filename, name)
	}
	return match[1], nil
}

const (
	// appFilenameRe is a regular expression template for an
	// application installer filename as stored in the hub
	appFilenameRe = "^%v-(.+)-linux-x86_64.tar$"
	// appPathRe is a regular expression template for a full
	// path to an application installer tarball in the hub
	appPathRe = "^%v/app/(.+)/(.+)/linux/x86_64/.+$"
)
