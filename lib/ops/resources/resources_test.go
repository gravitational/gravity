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

package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func TestResources(t *testing.T) { check.TestingT(t) }

type ResourceControlSuite struct{}

var _ = check.Suite(&ResourceControlSuite{})

func (s *ResourceControlSuite) TestResourceControl(c *check.C) {
	control := NewControl(&testResources{})

	reader := strings.NewReader(resources)

	err := control.Create(reader, false, "")
	c.Assert(err, check.IsNil)

	w := &bytes.Buffer{}
	err = control.Get(w, "", "", false, "text", "")
	c.Assert(err, check.IsNil)
	c.Assert(w.String(), check.Equals, `kind1/resource1
kind2/resource2
kind1/resource3
`)

	err = control.Remove("kind2", "resource2", false, "")
	c.Assert(err, check.IsNil)

	w.Reset()
	err = control.Get(w, "", "", false, "text", "")
	c.Assert(err, check.IsNil)
	c.Assert(w.String(), check.Equals, `kind1/resource1
kind1/resource3
`)
}

// testResources keeps created resources in memory
type testResources struct {
	resources []teleservices.UnknownResource
}

func (r *testResources) Create(req CreateRequest) error {
	r.resources = append(r.resources, req.Resource)
	return nil
}

func (r *testResources) GetCollection(req ListRequest) (Collection, error) {
	return testCollection(r.resources), nil
}

func (r *testResources) Remove(req RemoveRequest) error {
	for i, resource := range r.resources {
		if resource.Kind == req.Kind && resource.Metadata.Name == req.Name {
			r.resources = append(r.resources[:i], r.resources[i+1:]...)
			return nil
		}
	}
	return trace.NotFound("resource not found: %v", req)
}

// testCollection is a slice of test resources
type testCollection []teleservices.UnknownResource

func (c testCollection) Resources() ([]teleservices.UnknownResource, error) {
	return c, nil
}

func (c testCollection) WriteText(w io.Writer) error {
	var b bytes.Buffer
	for _, resource := range c {
		_, err := b.WriteString(fmt.Sprintf("%v/%v\n", resource.Kind, resource.Metadata.Name))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	_, err := w.Write(b.Bytes())
	return trace.Wrap(err)
}

func (c testCollection) WriteYAML(w io.Writer) error {
	bytes, err := yaml.Marshal(c)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(bytes)
	return trace.Wrap(err)
}

func (c testCollection) WriteJSON(w io.Writer) error {
	bytes, err := json.Marshal(c)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(bytes)
	return trace.Wrap(err)
}

const resources = `
kind: kind1
metadata:
  name: resource1
---
kind: kind2
metadata:
  name: resource2
---
kind: kind1
metadata:
  name: resource3
`
