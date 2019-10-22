/*
Copyright 2019 Gravitational, Inc.

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

package cli

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

func TestUtils(t *testing.T) { check.TestingT(t) }

type S struct{}

var _ = check.Suite(&S{})

func (*S) SetUpSuite(c *check.C) {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(os.Stderr)
}

func (*S) TestUpdatesCommandLine(c *check.C) {
	var testCases = []struct {
		comment     string
		inputArgs   []string
		flags       []flag
		removeFlags []string
		outputArgs  []string
	}{
		{
			comment:   "Does not overwrite existing flags",
			inputArgs: []string{"install", `--token=token`, "--debug"},
			flags: []flag{
				{
					name: "token", value: "different token",
				},
			},
			outputArgs: []string{
				"install", "--token", `"token"`, "--debug",
			},
		},
		{
			comment:   "Quotes flags and args",
			inputArgs: []string{"install", `--token=some token`, "/path/to/data", "--cloud-provider=generic"},
			flags: []flag{
				{
					name: "advertise-addr", value: "localhost:8080",
				},
			},
			removeFlags: []string{"cloud-provider"},
			outputArgs: []string{
				"install", "--token", `"some token"`, `"/path/to/data"`, "--advertise-addr", `"localhost:8080"`,
			},
		},
	}

	for _, testCase := range testCases {
		comment := check.Commentf(testCase.comment)
		args, err := updateCommandWithFlags(
			testCase.inputArgs,
			ArgsParserFunc(parseArgs),
			testCase.flags,
			testCase.removeFlags,
		)
		c.Assert(err, check.IsNil)
		c.Assert(args, check.DeepEquals, testCase.outputArgs, comment)
	}

}
