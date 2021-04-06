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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

func TestUtils(t *testing.T) { TestingT(t) }

type UtilsSuite struct{}

var _ = Suite(&UtilsSuite{})

func (s *UtilsSuite) TestEtcdInitialCluster(c *C) {
	memberListOutput := `6e3bd23ae5f1eae0: name=node2 peerURLs=http://2.2.2.2:23802 clientURLs=http://127.0.0.1:23792
924e2e83e93f2560: name=node3 peerURLs=https://3.3.3.3:23803 clientURLs=http://127.0.0.1:23793
a8266ecf031671f3: name=node1 peerURLs=http://1.1.1.1:23801 clientURLs=http://127.0.0.1:23791`

	out, err := EtcdInitialCluster(memberListOutput)
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "node2:2.2.2.2,node3:3.3.3.3,node1:1.1.1.1")
}

func (s *UtilsSuite) TestFindMemberID(c *C) {
	tcs := []struct {
		input string
		name  string
		id    string
		error bool
	}{
		{
			input: `
6e3bd23ae5f1eae0: name=node2 peerURLs=http://localhost:23802 clientURLs=http://127.0.0.1:23792
924e2e83e93f2560: name=node3 peerURLs=http://localhost:23803 clientURLs=http://127.0.0.1:23793
a8266ecf031671f3: name=node1 peerURLs=http://localhost:23801 clientURLs=http://127.0.0.1:23791
`,
			name: "node3",
			id:   "924e2e83e93f2560",
		},
		{
			input: `
6e3bd23ae5f1eae0 name=node2 peerURLs=http://localhost:23802 clientURLs=http://127.0.0.1:23792
`,
			name:  "node2",
			error: true,
		},
		{
			input: `
blablabla
`,
			error: true,
		},
	}
	for i, tc := range tcs {
		comment := Commentf("test case %v", i+1)
		id, err := FindETCDMemberID(tc.input, tc.name)
		if tc.error {
			c.Assert(err, NotNil, comment)
		} else {
			c.Assert(err, IsNil, comment)
			c.Assert(id, Equals, tc.id, comment)
		}
	}
}

// TestRetryReadOK makes sure that basic read works
func (s *UtilsSuite) TestRetryReadOK(c *C) {
	in := "hello, there!"
	var closer *testReadCloser
	rc, err := RetryRead(func() (io.ReadCloser, error) {
		closer = newTestReadCloser(bytes.NewBuffer([]byte(in)), 0)
		return closer, nil
	}, 0, 1)
	c.Assert(err, IsNil)
	defer rc.Close()

	out, err := ioutil.ReadAll(rc)
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, in)
	c.Assert(closer.closed, Equals, 1)
}

// TestRetryOnFailures tests the scenario when we've failed to
// read the contents several times. We also test that Close
// methods are closed at all times
func (s *UtilsSuite) TestRetryReadRetry(c *C) {
	in := "hello, there!"
	closer := newTestReadCloser(bytes.NewBuffer([]byte(in)), 2)
	rc, err := RetryRead(func() (io.ReadCloser, error) {
		return closer, nil
	}, 0, 3)
	c.Assert(err, IsNil)
	defer rc.Close()

	out, err := ioutil.ReadAll(rc)
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, in)
	c.Assert(closer.closed, Equals, 3)
}

// TestParseOpsCenterAddress
func (s *UtilsSuite) TestParseOpsCenterAddress(c *C) {
	tcs := []struct {
		input       string
		defaultPort string
		opscenter   string
	}{
		{
			input:       `example.com`,
			defaultPort: "443",
			opscenter:   "https://example.com:443",
		},
		{
			input:       `example.com:33009`,
			defaultPort: "443",
			opscenter:   "https://example.com:33009",
		},
		{
			input:       `example.com:33009/site`,
			defaultPort: "443",
			opscenter:   "https://example.com:33009",
		},
		{
			input:       `example.com`,
			defaultPort: "443",
			opscenter:   "https://example.com:443",
		},
		{
			input:       `https://example.com:443`,
			defaultPort: "33009",
			opscenter:   "https://example.com:443",
		},
		{
			input:       `https://example.com:33009`,
			defaultPort: "443",
			opscenter:   "https://example.com:33009",
		},
		{
			input:       `https://example.com:33009`,
			defaultPort: "443",
			opscenter:   "https://example.com:33009",
		},
		{
			input:       ``,
			defaultPort: "443",
			opscenter:   "",
		},
	}
	for i, tc := range tcs {
		comment := Commentf("test case %v", i+1)
		opscenter := ParseOpsCenterAddress(tc.input, tc.defaultPort)
		c.Assert(opscenter, Equals, tc.opscenter, comment)
	}
}

// TestParseDockerInfo parses Docker info command
func (s *UtilsSuite) TestParseDockerInfo(c *C) {
	tcs := []struct {
		input    string
		expected DockerInfo
		err      bool
	}{
		{
			input: `Containers: 468
 Running: 1
 Paused: 0
 Stopped: 467
Images: 1201
Server Version: 1.12.1
Storage Driver: aufs
 Root Dir: /var/lib/docker/aufs
 Backing Filesystem: extfs
 Dirs: 2043
 Dirperm1 Supported: true
Logging Driver: json-file
Cgroup Driver: cgroupfs
Plugins:
 Volume: local
 Network: bridge overlay null host
Swarm: inactive
Runtimes: runc
Default Runtime: runc
Security Options: apparmor
Kernel Version: 4.4.0-47-generic
Operating System: Ubuntu 16.04.1 LTS
OSType: linux
Architecture: x86_64
CPUs: 4
Total Memory: 31.29 GiB
Name: planet
ID: U2F3:ULBQ:HCCH:CSFQ:TFQA:HOPD:XGPG:5OA7:24RG:QKR3:IHHB:ZTVV
Docker Root Dir: /var/lib/docker
Debug Mode (client): false
Debug Mode (server): false
Registry: https://index.docker.io/v1/
WARNING: No swap limit support
Insecure Registries:
 127.0.0.0/8
`,
			expected: DockerInfo{ServerVersion: "1.12.1", StorageDriver: "aufs"},
		},
		{
			input: ``,
			err:   true,
		},
	}
	for i, tc := range tcs {
		comment := Commentf("test case %v", i+1)
		out, err := ParseDockerInfo(strings.NewReader(tc.input))
		if tc.err {
			c.Assert(err, NotNil, comment)
		} else {
			c.Assert(err, IsNil, comment)
			c.Assert(*out, DeepEquals, tc.expected)
		}
	}
}

type testReadCloser struct {
	io.Reader
	closed    int
	failCount int
}

func (t *testReadCloser) Close() error {
	t.closed++
	return nil
}

func (t *testReadCloser) Read(in []byte) (int, error) {
	if t.failCount > 0 {
		t.failCount--
		return 0, fmt.Errorf("fail: %v", t.failCount)
	}
	return t.Reader.Read(in)
}

func newTestReadCloser(r io.Reader, failCount int) *testReadCloser {
	return &testReadCloser{r, 0, failCount}
}

func (s *UtilsSuite) TestTrimPathPrefix(c *C) {
	tests := []struct {
		path   string
		prefix []string
		result string
	}{
		{
			path:   "/var/lib/gravity/resources/pod.yaml",
			prefix: []string{"/var/lib/gravity", "resources"},
			result: "pod.yaml",
		},
		{
			path:   "/var/lib/gravity/resources/pods/pod.yaml",
			prefix: []string{"/var/lib/gravity", "resources"},
			result: "pods/pod.yaml",
		},
		{
			path:   "/var/lib/gravity/resources/pods/pod.yaml",
			prefix: []string{"/var/lib/telekube"},
			result: "/var/lib/gravity/resources/pods/pod.yaml",
		},
	}
	for _, t := range tests {
		c.Assert(TrimPathPrefix(t.path, t.prefix...), Equals, t.result)
	}
}

func (s *UtilsSuite) TestSplitSlice(c *C) {
	tests := []struct {
		slice  []string
		size   int
		result [][]string
	}{
		{
			slice:  []string{"a", "b", "c", "d", "e"},
			size:   1,
			result: [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}},
		},
		{
			slice:  []string{"a", "b", "c", "d", "e"},
			size:   2,
			result: [][]string{{"a", "b"}, {"c", "d"}, {"e"}},
		},
		{
			slice:  []string{"a", "b", "c", "d", "e"},
			size:   3,
			result: [][]string{{"a", "b", "c"}, {"d", "e"}},
		},
		{
			slice:  []string{"a", "b", "c", "d", "e"},
			size:   5,
			result: [][]string{{"a", "b", "c", "d", "e"}},
		},
		{
			slice:  []string{"a", "b", "c", "d", "e"},
			size:   250,
			result: [][]string{{"a", "b", "c", "d", "e"}},
		},
	}
	for _, test := range tests {
		c.Assert(SplitSlice(test.slice, test.size), DeepEquals, test.result)
	}
}
