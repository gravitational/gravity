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

package suite

import (
	"bytes"
	"io/ioutil"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1" //nolint:revive,stylecheck // TODO: tests will be rewritten to use testify
)

type BLOBSuite struct {
	Objects blob.Objects
}

// BLOB is a test for BLOB storage
func (s *BLOBSuite) BLOB(c *C) {
	blob1 := "hello, blob 1"
	e, err := s.Objects.WriteBLOB(bytes.NewBuffer([]byte(blob1)))
	c.Assert(err, IsNil)
	c.Assert(e.SizeBytes, Equals, int64(len(blob1)))
	c.Assert(e.SHA512, Equals, utils.MustSHA512Half([]byte(blob1)))

	r, err := s.Objects.OpenBLOB(e.SHA512)
	c.Assert(err, IsNil)

	out, err := ioutil.ReadAll(r)
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, blob1)
	c.Assert(r.Close(), IsNil)
	// second close should not panic or freeze
	r.Close()

	envelope, err := s.Objects.GetBLOBEnvelope(e.SHA512)
	c.Assert(err, IsNil)
	c.Assert(envelope, DeepEquals, e)

	err = s.Objects.DeleteBLOB(e.SHA512)
	c.Assert(err, IsNil)

	_, err = s.Objects.OpenBLOB(e.SHA512)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("type: %#v", err))
}

// BLOBList is a test for BLOB storage
func (s *BLOBSuite) BLOBList(c *C) {
	blob1 := "hello, blob 1"
	e, err := s.Objects.WriteBLOB(bytes.NewBuffer([]byte(blob1)))
	c.Assert(err, IsNil)
	c.Assert(e.SizeBytes, Equals, int64(len(blob1)))
	c.Assert(e.SHA512, Equals, utils.MustSHA512Half([]byte(blob1)))

	out, err := s.Objects.GetBLOBs()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []string{e.SHA512})

	blob2 := "hello, blob 2"
	e2, err := s.Objects.WriteBLOB(bytes.NewBuffer([]byte(blob2)))
	c.Assert(err, IsNil)

	out, err = s.Objects.GetBLOBs()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []string{e.SHA512, e2.SHA512})

	err = s.Objects.DeleteBLOB(e.SHA512)
	c.Assert(err, IsNil)

	out, err = s.Objects.GetBLOBs()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []string{e2.SHA512})
}

// BLOBSeek tests that seek works
func (s *BLOBSuite) BLOBSeek(c *C) {
	blob1 := "hello, blob 1"
	e, err := s.Objects.WriteBLOB(bytes.NewBuffer([]byte(blob1)))
	c.Assert(err, IsNil)

	r, err := s.Objects.OpenBLOB(e.SHA512)
	c.Assert(err, IsNil)
	defer r.Close()

	out, err := ioutil.ReadAll(r)
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, blob1)

	_, err = r.Seek(0, 0)
	c.Assert(err, IsNil)

	out2, err := ioutil.ReadAll(r)
	c.Assert(err, IsNil)
	c.Assert(string(out2), Equals, blob1)
}

// BLOBWriteTwice tests that writing twice same data produces same BLOB
func (s *BLOBSuite) BLOBWriteTwice(c *C) {
	blob1 := "hello, blob 1"
	_, err := s.Objects.WriteBLOB(bytes.NewBuffer([]byte(blob1)))
	c.Assert(err, IsNil)

	e, err := s.Objects.WriteBLOB(bytes.NewBuffer([]byte(blob1)))
	c.Assert(err, IsNil)

	r, err := s.Objects.OpenBLOB(e.SHA512)
	c.Assert(err, IsNil)
	defer r.Close()

	out, err := ioutil.ReadAll(r)
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, blob1)
}
