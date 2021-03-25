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

package testutils

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	check "gopkg.in/check.v1"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// S3 is the mocked S3 API client
type S3 struct {
	s3iface.S3API
	// Objects is the objects stored in the fake S3
	Objects map[string]S3Object
}

// S3Object represents a file object stored in the fake S3
type S3Object struct {
	// Data is the file data
	Data []byte
	// Created is the file creation timestamp
	Created time.Time
}

// S3App represents a test application
type S3App struct {
	// Name of the application name
	Name string
	// Version is the application version
	Version string
	// Created is the application creation timestamp
	Created time.Time
	// Data is the application data
	Data []byte
	// Checksum is the application file sha256 checksum
	Checksum string
}

// NewS3 returns a new fake S3 implementation
func NewS3() *S3 {
	return &S3{
		Objects: make(map[string]S3Object),
	}
}

// Add adds the provided application to the hub
func (s *S3) Add(c *check.C, app S3App, options ...AddOption) {
	key := fmt.Sprintf("%v/app/%v/%v/linux/x86_64/%v-%v-linux-x86_64.tar",
		defaults.HubTelekubePrefix, app.Name, app.Version, app.Name, app.Version)
	s.Objects[key] = S3Object{Data: app.Data, Created: app.Created}
	s.Objects[key+".sha256"] = S3Object{Data: []byte(app.Checksum), Created: app.Created}
	for _, option := range options {
		option(s, app)
	}
	s.addToIndex(c, app)
}

func (s *S3) addToIndex(c *check.C, app S3App) {
	indexFile := repo.NewIndexFile()
	key := fmt.Sprintf("%v/index.yaml", defaults.HubTelekubePrefix)
	if o, ok := s.Objects[key]; ok {
		err := yaml.Unmarshal(o.Data, indexFile)
		c.Assert(err, check.IsNil)
	}
	indexFile.Add(&chart.Metadata{
		Name:    app.Name,
		Version: app.Version,
		Annotations: map[string]string{
			constants.AnnotationSize: fmt.Sprintf("%v", len(app.Data)),
		},
	}, "", "", app.Checksum)
	bytes, err := yaml.Marshal(indexFile)
	c.Assert(err, check.IsNil)
	s.Objects[key] = S3Object{Data: bytes}
}

// AddOption represents an object add option
type AddOption func(*S3, S3App)

// WithLatest adds the provided app to the latest bucket
func WithLatest() AddOption {
	return func(s *S3, app S3App) {
		key := fmt.Sprintf("%v/app/%v/latest/linux/x86_64/%v-%v-linux-x86_64.tar",
			defaults.HubTelekubePrefix, app.Name, app.Name, app.Version)
		s.Objects[key] = S3Object{Data: app.Data, Created: app.Created}
		s.Objects[key+".sha256"] = S3Object{Data: []byte(app.Checksum), Created: app.Created}
	}
}

// WithStable adds the provided app to the stable bucket
func WithStable() AddOption {
	return func(s *S3, app S3App) {
		key := fmt.Sprintf("%v/app/%v/stable/linux/x86_64/%v-%v-linux-x86_64.tar",
			defaults.HubTelekubePrefix, app.Name, app.Name, app.Version)
		s.Objects[key] = S3Object{Data: app.Data, Created: app.Created}
		s.Objects[key+".sha256"] = S3Object{Data: []byte(app.Checksum), Created: app.Created}
	}
}

func (s *S3) ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	var objects []*s3.Object
	for key, object := range s.Objects {
		if *input.Prefix != "" && !strings.HasPrefix(key, *input.Prefix) {
			continue
		}
		objects = append(objects, &s3.Object{
			Key:          aws.String(key),
			LastModified: aws.Time(object.Created),
			Size:         aws.Int64(int64(len(object.Data))),
		})
	}
	return &s3.ListObjectsV2Output{
		Contents: objects,
		KeyCount: aws.Int64(int64(len(objects))),
		Name:     aws.String(defaults.HubBucket),
		Prefix:   input.Prefix,
	}, nil
}

func (s *S3) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	return s.GetObjectWithContext(context.TODO(), input)
}

func (s *S3) GetObjectWithContext(ctx aws.Context, input *s3.GetObjectInput, options ...request.Option) (*s3.GetObjectOutput, error) {
	object, ok := s.Objects[aws.StringValue(input.Key)]
	if !ok {
		return nil, trace.NotFound("key %v not found", aws.StringValue(input.Key))
	}
	return &s3.GetObjectOutput{
		Body:          ioutil.NopCloser(bytes.NewBuffer(object.Data)),
		ContentLength: aws.Int64(int64(len(object.Data))),
	}, nil
}
