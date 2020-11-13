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

package systemservice

import (
	"bytes"
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/loc"

	. "gopkg.in/check.v1"
)

func TestSystemd(t *testing.T) { TestingT(t) }

type SystemdSuite struct {
}

var _ = Suite(&SystemdSuite{})

func (s *SystemdSuite) TestUnitParsing(c *C) {
	pkg, err := loc.NewLocator("example.com", "package", "0.0.1")
	c.Assert(err, IsNil)
	u := newSystemdUnit(*pkg)
	c.Assert(u.serviceName(), Equals, "gravity__example.com__package__0.0.1.service")
	out := parseUnit(u.serviceName())
	c.Assert(out, NotNil)
	c.Assert(out.IsEqualTo(*pkg), Equals, true)

	c.Assert(parseUnit("other.service"), IsNil)
}

func (s *SystemdSuite) TestServiceTemplate(c *C) {
	tt := []struct {
		in          serviceTemplate
		out         string
		description string
	}{
		{
			description: "Full service",
			in: serviceTemplate{
				Name:        "test.service",
				Description: "test",
				ServiceSpec: ServiceSpec{
					Type:             "oneshot",
					StartCommand:     "start",
					StopCommand:      "stop",
					StopPostCommand:  "stop post",
					StartPreCommands: []string{"pre-command", "another pre-command"},
					StartPostCommand: "start post",
					WantedBy:         "test.target",
					KillMode:         "cgroup",
					KillSignal:       "SIGQUIT",
					RestartSec:       3,
					Timeout:          4,
					Restart:          "always",
					User:             "root",
					LimitNoFile:      1000,
					RemainAfterExit:  true,
					Dependencies: Dependencies{
						Requires: "foo.service",
						After:    "foo.service",
						Before:   "bar.service",
					},
					Environment: map[string]string{
						"PATH": "/usr/bin",
					},
					TasksMax:                 "infinity",
					TimeoutStopSec:           "5min",
					ConditionPathExists:      "/path/to/foo",
					RestartPreventExitStatus: "1 2 3",
					SuccessExitStatus:        "254",
					WorkingDirectory:         "/foo/bar",
				},
			},
			out: `[Unit]
Description=test

Requires=foo.service
After=foo.service
Before=bar.service

ConditionPathExists=/path/to/foo

[Service]
TimeoutStartSec=4
Type=oneshot
User=root
ExecStart=start
ExecStartPre=pre-command
ExecStartPre=another pre-command
ExecStartPost=start post
ExecStop=stop
ExecStopPost=stop post
LimitNOFILE=1000
KillMode=cgroup
KillSignal=SIGQUIT
Restart=always
TimeoutStopSec=5min
RestartSec=3
RemainAfterExit=yes
RestartPreventExitStatus=1 2 3
SuccessExitStatus=254
WorkingDirectory=/foo/bar
Environment=PATH=/usr/bin

TasksMax=infinity


[Install]
WantedBy=test.target

`,
		},
		{
			description: "unspecified file limits",
			in: serviceTemplate{
				Name:        "test.service",
				Description: "test",
				ServiceSpec: ServiceSpec{
					Type:             "oneshot",
					StartCommand:     "start",
					StopCommand:      "stop",
					StopPostCommand:  "stop post",
					StartPreCommands: []string{"pre-command", "another pre-command"},
					StartPostCommand: "start post",
					WantedBy:         "test.target",
					KillMode:         "cgroup",
					KillSignal:       "SIGQUIT",
					RestartSec:       3,
					Timeout:          4,
					Restart:          "always",
					User:             "root",
					RemainAfterExit:  true,
					Dependencies: Dependencies{
						Requires: "foo.service",
						After:    "foo.service",
						Before:   "bar.service",
					},
					Environment: map[string]string{
						"PATH": "/usr/bin",
					},
					TasksMax:                 "infinity",
					TimeoutStopSec:           "5min",
					ConditionPathExists:      "/path/to/foo",
					RestartPreventExitStatus: "1 2 3",
					SuccessExitStatus:        "254",
					WorkingDirectory:         "/foo/bar",
				},
			},
			out: `[Unit]
Description=test

Requires=foo.service
After=foo.service
Before=bar.service

ConditionPathExists=/path/to/foo

[Service]
TimeoutStartSec=4
Type=oneshot
User=root
ExecStart=start
ExecStartPre=pre-command
ExecStartPre=another pre-command
ExecStartPost=start post
ExecStop=stop
ExecStopPost=stop post
LimitNOFILE=100000
KillMode=cgroup
KillSignal=SIGQUIT
Restart=always
TimeoutStopSec=5min
RestartSec=3
RemainAfterExit=yes
RestartPreventExitStatus=1 2 3
SuccessExitStatus=254
WorkingDirectory=/foo/bar
Environment=PATH=/usr/bin

TasksMax=infinity


[Install]
WantedBy=test.target

`,
		},
	}

	for _, test := range tt {
		buf := &bytes.Buffer{}
		err := serviceUnitTemplate.Execute(buf, test.in)
		c.Assert(err, IsNil)
		c.Assert(buf.String(), compare.DeepEquals, test.out, Commentf(test.description))
	}
}

func (s *SystemdSuite) TestMountServiceTemplate(c *C) {
	var testCases = []struct {
		spec     MountServiceSpec
		expected string
		comment  string
	}{
		{
			spec: MountServiceSpec{
				What:       "/dev/foo",
				Where:      "/foo/bar",
				Type:       "filesystem",
				Options:    []string{"opt1", "opt2"},
				TimeoutSec: "5min 20s",
			},
			expected: `
[Mount]
What=/dev/foo
Where=/foo/bar
Type=filesystem
Options=opt1,opt2
TimeoutSec=5min 20s

[Install]
WantedBy=local-fs.target
`,
			comment: "formats all details",
		},
		{
			spec: MountServiceSpec{
				What:  "/dev/foo",
				Where: "/foo/bar",
			},
			expected: `
[Mount]
What=/dev/foo
Where=/foo/bar




[Install]
WantedBy=local-fs.target
`,
			comment: "leaves out optional parts",
		},
	}

	for _, testCase := range testCases {
		var out bytes.Buffer
		err := mountUnitTemplate.Execute(&out, testCase.spec)
		c.Assert(err, IsNil, Commentf(testCase.comment))
		c.Assert(out.String(), compare.DeepEquals, testCase.expected, Commentf(testCase.comment))
	}
}
