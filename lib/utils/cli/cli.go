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
	// FlagsToRemove is a list of flags to omit from the resulting command.
	FlagsToRemove []string
}

// Update returns new command line for the provided command taking into account
// flags that need to be added or removed as configured.
// Positional arguments are moved to the end.
//
// If a flag needs to be replaced (possibly to update the value), it needs to be
// placed into both FlagsToAdd and FlagsToRemove.
//
// The resulting command line adheres to command line format accepted by systemd.
// See https://www.freedesktop.org/software/systemd/man/systemd.service.html#Command%20lines for details
func (r *CommandArgs) Update(command []string) (args []string, err error) {
	ctx, err := r.Parser.ParseArgs(command)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse command: %v", command)
	}
	for _, flag := range r.addRemoveFlags(ctx) {
		args = append(args, flag.Format()...)
	}
	outputCommand := ctx.SelectedCommand.FullCommand()
	return append([]string{outputCommand}, args...), nil
}

func (r *CommandArgs) addRemoveFlags(ctx *kingpin.ParseContext) (flags []Flag) {
	var args []Flag
	seen := make(map[string]struct{})
	for _, el := range ctx.Elements {
		switch c := el.Clause.(type) {
		case *kingpin.ArgClause:
			if utils.StringInSlice(r.FlagsToRemove, c.Model().Name) {
				// Remove the positional argument
				continue
			}
			seen[c.Model().Name] = struct{}{}
			args = append(args, NewArg(c.Model().Name, *el.Value))
		case *kingpin.FlagClause:
			if utils.StringInSlice(r.FlagsToRemove, c.Model().Name) {
				// Remove the flag
				continue
			}
			seen[c.Model().Name] = struct{}{}
			if _, ok := c.Model().Value.(boolCmdlineFlag); ok {
				flags = append(flags, newBoolFlag(c.Model().Name, *el.Value))
			} else {
				flags = append(flags, NewFlag(c.Model().Name, *el.Value))
			}
		}
	}
	for _, flag := range r.FlagsToAdd {
		if arg, ok := flag.(arg); ok {
			if _, exists := seen[arg.name]; !exists {
				args = append(args, arg)
			}
			continue
		}
		if _, exists := seen[flag.Name()]; !exists {
			flags = append(flags, flag)
		}
	}
	// Return the new command line with positional arguments following non-positional flags
	return append(flags, args...)
}

// NewArg creates a new positional argument
func NewArg(name, value string) Flag {
	return arg{name: name, value: value}
}

// Name returns the argument's name
func (r arg) Name() string {
	return r.name
}

// Formats returns this argument formatted for command line
func (r arg) Format() []string {
	return []string{fmt.Sprint(strconv.Quote(r.value))}
}

type arg struct {
	name  string
	value string
}

// Flag represents a command-line flag
type Flag interface {
	// Format formats the flag for command line
	Format() []string
	// Name returns the flag's name
	Name() string
}

// NewFlag creates a new named command-line option with a value.
func NewFlag(name, value string) Flag {
	return stringFlag{name: name, value: value}
}

// Name returns the flag's name
func (r stringFlag) Name() string {
	return r.name
}

// Formats returns this flag formatted for command line
func (r stringFlag) Format() []string {
	return []string{fmt.Sprint("--", r.name), strconv.Quote(r.value)}
}

// stringFlag represents a named command-line flag with a value.
type stringFlag struct {
	// name is the flag name.
	name string
	// value is the flag value.
	value string
}

// NewBoolFlag creates a new boolean command-line flag.
func NewBoolFlag(name string, value bool) Flag {
	return boolFlag{name: name, value: value}
}

// Name returns this flag's name
func (r boolFlag) Name() string {
	return r.name
}

// Formats returns this flag formatted for command line
func (r boolFlag) Format() []string {
	if r.value {
		return []string{fmt.Sprint("--", r.name)}
	}
	return []string{fmt.Sprint("--no-", r.name)}
}

func newBoolFlag(name, value string) Flag {
	return boolFlag{name: name, value: value == "true"}
}

// boolFlag represents a boolean command-line flag.
// Boolean flag does not have a value and can be flipped
// by prefixing it with a 'no-' prefix:
//
//  --bool-value	to enable bool-value
//  --no-bool-value	to disale bool-value
type boolFlag struct {
	// name is the flag name.
	name string
	// value is the flag value.
	value bool
}

// boolCmdlineFlag describes a boolean commane-line flag.
// It exists to support kingpin boolean flags since the package itself
// does not export the type
type boolCmdlineFlag interface {
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
