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
	"fmt"
	"os"
	"testing"

	"github.com/gravitational/gravity/lib/constants"

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
		comment      string
		inputArgs    []string
		flags        []Flag
		replaceFlags []Flag
		removeFlags  []string
		outputArgs   []string
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
			inputArgs: []string{"install", "--no-selinux", "/path/to/data"},
			outputArgs: []string{
				"install", "--no-selinux", `"/path/to/data"`,
			},
		},
		{
			comment: "Replaces boolean flag with opposite value",
			// selinux is on by default
			inputArgs: []string{"install", "/path/to/data"},
			outputArgs: []string{
				"install", "--no-selinux", `"/path/to/data"`,
			},
			flags: []Flag{
				NewBoolFlag("selinux", false),
			},
			removeFlags: []string{"selinux"},
		},
		{
			comment:    "Redact flags",
			inputArgs:  []string{"install", `--token=token`, "--debug"},
			outputArgs: []string{"install", "--token", fmt.Sprintf(`"%s"`, constants.Redacted), "--debug"},
			replaceFlags: []Flag{
				NewFlag("token", constants.Redacted),
			},
		},
	}

	for _, testCase := range testCases {
		comment := check.Commentf(testCase.comment)
		commandArgs := CommandArgs{
			Parser:         ArgsParserFunc(parseArgs),
			FlagsToAdd:     testCase.flags,
			FlagsToRemove:  testCase.removeFlags,
			FlagsToReplace: testCase.replaceFlags,
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
	cmd.Flag("selinux", "").Default("true").Bool()
	cmd.Flag("advertise-addr", "").String()
	cmd.Flag("cloud-provider", "").String()
	return app.ParseContext(args)
}
