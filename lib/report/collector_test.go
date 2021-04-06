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

package report

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/gravitational/gravity/lib/utils"

	. "gopkg.in/check.v1"
)

type S struct {
	dir string
}

func TestS(t *testing.T) { TestingT(t) }

var _ = Suite(&S{})

func (r *S) SetUpSuite(c *C) {
	r.dir = c.MkDir()
}

func (r *S) TestPendingWriterDoesNotCreateZeroSizeFile(c *C) {
	path := r.path("a")
	w := NewPendingFileWriter(path)
	// No writes
	err := w.Close()
	c.Assert(err, IsNil)

	_, err = utils.StatFile(path)
	c.Assert(err, ErrorMatches, ".*no such file.*")
}

func (r *S) TestPendingWriterProperlyCreatesFileWithData(c *C) {
	var contents = []byte("brown fox jumps over the lazy dog")

	path := r.path("b")
	w := NewPendingFileWriter(path)
	_, err := w.Write(contents)
	c.Assert(err, IsNil)
	err = w.Close()
	c.Assert(err, IsNil)

	out, err := ioutil.ReadFile(path)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, contents)
}

func (r *S) path(subdirs ...string) string {
	return filepath.Join(append([]string{r.dir}, subdirs...)...)
}
