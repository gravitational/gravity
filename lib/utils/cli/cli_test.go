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
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/check.v1"
)

func TestCLI(t *testing.T) { check.TestingT(t) }

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
		flags       []Flag
		removeFlags []string
		outputArgs  []string
	}{
		{
			comment:   "Does not overwrite existing flags",
			inputArgs: []string{"install", `--token=token`, "--debug"},
			flags:     []Flag{NewFlag("token", "different token")},
			outputArgs: []string{
				"install", "--token", `"token"`, "--debug",
			},
		},
		{
			comment:     "Quotes flags and args",
			inputArgs:   []string{"install", `--token=some token`, "/path/to/data", "--cloud-provider=generic"},
			flags:       []Flag{NewFlag("advertise-addr", "localhost:8080")},
			removeFlags: []string{"cloud-provider"},
			outputArgs: []string{
				"install", "--token", `"some token"`, "--advertise-addr", `"localhost:8080"`, `"/path/to/data"`,
			},
		},
		{
			comment:   "Handles negated flags",
			inputArgs: []string{"install", "--no-debug", "/path/to/data"},
			outputArgs: []string{
				"install", "--no-debug", `"/path/to/data"`,
			},
		},
		{
			comment: "Replaces boolean flag with opposite value",
			// debug is off by default
			inputArgs: []string{"install", "/path/to/data"},
			outputArgs: []string{
				"install", "--debug", `"/path/to/data"`,
			},
			flags: []Flag{
				NewBoolFlag("debug", true),
			},
			removeFlags: []string{"debug"},
		},
		{
			comment:   "Can update existing positional argument",
			inputArgs: []string{"install", "/path/to/data"},
			outputArgs: []string{
				"install", `"/path/to/data"`,
			},
			flags: []Flag{
				NewArg("path", "/path/to/data"),
			},
			removeFlags: []string{"path"},
		},
		{
			comment:   "Adds implicit positional argument",
			inputArgs: []string{"install"},
			outputArgs: []string{
				"install", `"/path/to/data"`,
			},
			flags: []Flag{
				NewArg("path", "/path/to/data"),
			},
			removeFlags: []string{"path"},
		},
	}

	for _, testCase := range testCases {
		comment := check.Commentf(testCase.comment)
		commandArgs := CommandArgs{
			Parser:        ArgsParserFunc(parseArgs),
			FlagsToAdd:    testCase.flags,
			FlagsToRemove: testCase.removeFlags,
		}
		args, err := commandArgs.Update(testCase.inputArgs)
		c.Assert(err, check.IsNil)
		c.Assert(args, check.DeepEquals, testCase.outputArgs, comment)
	}
}

func parseArgs(args []string) (*kingpin.ParseContext, error) {
	app := kingpin.New("test", "")
	app.Flag("debug", "").Bool()
	cmd := app.Command("install", "")
	cmd.Arg("path", "").String()
	cmd.Flag("token", "").String()
	cmd.Flag("advertise-addr", "").String()
	cmd.Flag("cloud-provider", "").String()
	return app.ParseContext(args)
}
