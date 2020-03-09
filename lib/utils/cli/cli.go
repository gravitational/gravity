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
	"strconv"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"gopkg.in/alecthomas/kingpin.v2"
)

// CommandArgs manipulates command-line arguments.
type CommandArgs struct {
	// Parser is used to parse provided command-line.
	Parser ArgsParser
	// FlagsToAdd is a list of additional flags to add to resulting command.
	FlagsToAdd []Flag
	// FlagsToRemote is a lit of flags to omit from the resulting command.
	FlagsToRemove []string
}

// Update returns new command line for the provided command taking into account
// flags that need to be added or removed as configured.
//
// The resulting command line adheres to command line format accepted by systemd.
// See https://www.freedesktop.org/software/systemd/man/systemd.service.html#Command%20lines for details
func (a *CommandArgs) Update(command []string, flagsToAdd ...Flag) (args []string, err error) {
	ctx, err := a.Parser.ParseArgs(command)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse command: %v", command)
	}
	seen := make(map[string]struct{})
	for _, el := range ctx.Elements {
		switch c := el.Clause.(type) {
		case *kingpin.ArgClause:
			args = append(args, strconv.Quote(*el.Value))
		case *kingpin.FlagClause:
			if utils.StringInSlice(a.FlagsToRemove, c.Model().Name) {
				continue
			}
			if _, ok := c.Model().Value.(boolFlag); ok {
				switch *el.Value {
				case "true":
					args = append(args, fmt.Sprint("--", c.Model().Name))
				case "false":
					args = append(args, fmt.Sprint("--no-", c.Model().Name))
				}
			} else {
				args = append(args, fmt.Sprint("--", c.Model().Name), strconv.Quote(*el.Value))
			}
			seen[c.Model().Name] = struct{}{}
		}
	}
	for _, flag := range append(a.FlagsToAdd, flagsToAdd...) {
		if _, ok := seen[flag.Name]; !ok {
			args = append(args, fmt.Sprint("--", flag.Name), strconv.Quote(flag.Value))
		}
	}
	outputCommand := ctx.SelectedCommand.FullCommand()
	return append([]string{outputCommand}, args...), nil
}

// Flag represents a single command-line flag.
type Flag struct {
	// Name is the flag name.
	Name string
	// Value is the flag value.
	Value string
}

// NewFlag creates a new flag.
func NewFlag(name, value string) Flag {
	return Flag{Name: name, Value: value}
}

type boolFlag interface {
	// IsBoolFlag returns true to indicate a boolean flag.
	IsBoolFlag() bool
}

// ParseArgs parses the specified command line arguments into a parse context.
func (r ArgsParserFunc) ParseArgs(args []string) (*kingpin.ParseContext, error) {
	return r(args)
}

// ArgsParserFunc is a functional wrapper for ArgsParser to enable ordinary
// functions as ArgsParsers.
type ArgsParserFunc func(args []string) (*kingpin.ParseContext, error)

// ArgsParser parses command line arguments.
type ArgsParser interface {
	// ParseArgs parses the specified command line arguments into a parse context.
	ParseArgs(args []string) (*kingpin.ParseContext, error)
}
