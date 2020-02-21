/*
Copyright 2020 Gravitational, Inc.

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

package selinux

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	liblog "github.com/gravitational/gravity/lib/log"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/satellite/monitoring"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestSuite(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) TestWritesBootstrapScript(c *C) {
	var testCases = []struct {
		config   BootstrapConfig
		expected string
		fcontext string
		comment  string
	}{
		{
			config: BootstrapConfig{
				OS:   &testSystem,
				Path: "/path/to/installer",
			},
			expected: `
port -D
fcontext -D
fcontext -a -f f -t gravity_installer_exec_t -r 's0' '/path/to/installer/gravity'
fcontext -a -f f -t gravity_log_t -r 's0' '/path/to/installer/gravity-(install|system)\.log'
fcontext -a -f a -t gravity_home_t -r 's0' '/path/to/installer/.gravity(/.*)?'
`,
			comment: "does nothing for custom fcontext if no custom state directory",
		},
		{
			config: BootstrapConfig{
				OS:       &testSystem,
				Path:     "/path/to/installer",
				StateDir: "/custom/state/dir",
			},
			fcontext: `
# Custom fcontext entries

/custom/state/dir/file -- gen_context(system_u:object_r:file_type_t, s0)
/custom/state/dir/dir -d gen_context(system_u:object_r:dir_type_t, s0)
/custom/state/dir/socket.* -s gen_context(system_u:object_r:socket_type_t,s0)
/custom/state/dir/symlink -l gen_context(system_u:object_r:symlink_type_t,s0)
/custom/state/dir/pipe -p gen_context(system_u:object_r:pipe_type_t, s0)
/custom/state/dir/bdev -b gen_context(system_u:object_r:bdev_type_t, s0)
/custom/state/dir/cdev -c gen_context(system_u:object_r:cdev_type_t, s0)
/custom/state/dir/dir2(/.*)? 	gen_context(system_u:object_r:file_type_t,	s0)
/custom/state/dir3(/.*)? 	<<none>>
`,
			expected: `
port -D
fcontext -D
fcontext -a -f f -t gravity_installer_exec_t -r 's0' '/path/to/installer/gravity'
fcontext -a -f f -t gravity_log_t -r 's0' '/path/to/installer/gravity-(install|system)\.log'
fcontext -a -f a -t gravity_home_t -r 's0' '/path/to/installer/.gravity(/.*)?'
fcontext --add --ftype f --type file_type_t --range 's0' '/custom/state/dir/file'
fcontext --add --ftype d --type dir_type_t --range 's0' '/custom/state/dir/dir'
fcontext --add --ftype s --type socket_type_t --range 's0' '/custom/state/dir/socket.*'
fcontext --add --ftype l --type symlink_type_t --range 's0' '/custom/state/dir/symlink'
fcontext --add --ftype p --type pipe_type_t --range 's0' '/custom/state/dir/pipe'
fcontext --add --ftype b --type bdev_type_t --range 's0' '/custom/state/dir/bdev'
fcontext --add --ftype c --type cdev_type_t --range 's0' '/custom/state/dir/cdev'
fcontext --add --ftype a --type file_type_t --range 's0' '/custom/state/dir/dir2(/.*)?'

`,
			comment: "includes custom fcontext entries for a custom state directory",
		},
		{
			config: BootstrapConfig{
				OS:         &testSystem,
				Path:       "/path/to/installer",
				StateDir:   "/custom/state/dir",
				portRanges: newPortRanges(),
			},
			expected: `
port -D
fcontext -D
port -a -t gravity_install_port_t -r 's0' -p tcp 1000-1001
port -a -t gravity_kubernetes_port_t -r 's0' -p udp 2000-2000
port -a -t gravity_port_t -r 's0' -p tcp 3000-3000
port -a -t gravity_port_t -r 's0' -p udp 4000-4002
fcontext -a -f f -t gravity_installer_exec_t -r 's0' '/path/to/installer/gravity'
fcontext -a -f f -t gravity_log_t -r 's0' '/path/to/installer/gravity-(install|system)\.log'
fcontext -a -f a -t gravity_home_t -r 's0' '/path/to/installer/.gravity(/.*)?'
`,
			comment: "overrides vxlan port",
		},
	}
	for _, testCase := range testCases {
		comment := Commentf(testCase.comment)
		b := newTestBootstrapper(testCase.config, testCase.fcontext, "TestWritesBootstrapScript")
		var buf bytes.Buffer
		err := b.writeBootstrapScript(&buf)
		c.Assert(err, IsNil, comment)
		c.Assert(buf.String(), Equals, testCase.expected, comment)
	}
}

func newTestBootstrapper(config BootstrapConfig, s, testCase string) *bootstrapper {
	return &bootstrapper{
		logger:           liblog.New(log.WithField(trace.Component, testCase)),
		config:           config,
		policyFileReader: policyFileFromLiteral("distro/gravity.statedir.fc.template", s),
	}
}

func newPortRanges() portRanges {
	return portRanges{
		Installer: []schema.PortRange{
			{
				Protocol:    "tcp",
				From:        1000,
				To:          1001,
				Description: "installer port",
			},
		},
		Kubernetes: []schema.PortRange{
			{
				Protocol:    "udp",
				From:        2000,
				To:          2000,
				Description: "kubernetes service port",
			},
		},
		Generic: []schema.PortRange{
			{
				Protocol:    "tcp",
				From:        3000,
				To:          3000,
				Description: "gravity service port",
			},
			{
				Protocol:    "udp",
				From:        4000,
				To:          4002,
				Description: "another gravity service port",
			},
		},
	}
}

func policyFileFromLiteral(fname, content string) policyFileReaderFunc {
	return func(name string) (io.ReadCloser, error) {
		if fname != name {
			return nil, trace.NotFound("file %q not found", name)
		}
		return ioutil.NopCloser(strings.NewReader(content)), nil
	}
}

var testSystem = monitoring.OSRelease{
	ID: "distro",
}
