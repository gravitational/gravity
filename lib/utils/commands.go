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
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/trace"
)

// PlanetCommandArgs returns a new command to run the command specified with args
// inside planet container.
// For details of operation see PlanetCommandSlice
func PlanetCommandArgs(args ...string) []string {
	return Exe.PlanetCommandSlice(args)
}

// PlanetCommand returns a new command to run the specified command cmd
// inside planet container.
func PlanetCommand(cmd Command) []string {
	return Exe.PlanetCommandSlice(cmd.Args())
}

// PlanetCommandSlice returns a new command to run the command specified with
// args inside planet.
// If the process is already running inside the container, the command
// is returned unaltered.
// gravityArgs optionally specify additional arguments to the gravity binary
// The command is using the path of the currently running process for the fork call.
func PlanetCommandSlice(args []string, gravityArgs ...string) []string {
	return Exe.PlanetCommandSlice(args, gravityArgs...)
}

// PlanetEnterCommand returns command that runs in planet using gravity from path
func PlanetEnterCommand(args ...string) []string {
	return append([]string{constants.GravityBin,
		"exec", "--no-tty", "--no-interactive"}, args...)
}

// Self returns the command line for the currently running executable.
// args specifies additional command line arguments
func Self(args ...string) []string {
	return Exe.Self(args...)
}

// Exe is the Executable for the currently running gravity binary
var Exe = must(NewCurrentExecutable())

// PlanetCommandArgs returns a new command to run the command specified with args
// inside planet container.
// For details of operation see PlanetCommandSlice
func (r Executable) PlanetCommandArgs(args ...string) []string {
	return r.PlanetCommandSlice(args)
}

// PlanetCommand returns a new command to run the specified command cmd
// inside planet container.
func (r Executable) PlanetCommand(cmd Command) []string {
	return r.PlanetCommandSlice(cmd.Args())
}

// PlanetCommandSlice returns a new command to run the command specified with
// args inside planet.
// If the process is already running inside the container, the command
// is returned unaltered.
// gravityArgs optionally specify additional arguments to the gravity binary
func (r Executable) PlanetCommandSlice(args []string, gravityArgs ...string) []string {
	if CheckInPlanet() {
		return args
	}

	command := args[0]
	commandArgs := args[1:]

	gravityCommand := []string{r.Path}
	gravityCommand = append(gravityCommand, gravityArgs...)
	gravityCommand = append(gravityCommand,
		"exec", "--no-tty", "--no-interactive", command)
	gravityCommand = append(gravityCommand, commandArgs...)

	return gravityCommand
}

// Self returns the command line for the currently running executable.
// args specifies additional command line arguments
func (r Executable) Self(args ...string) []string {
	return append([]string{r.Path}, args...)
}

// NewCurrentExecutable returns a new Executable for the currently running gravity binary
func NewCurrentExecutable() (*Executable, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return &Executable{
		Path:       path,
		WorkingDir: wd,
	}, nil
}

// Executable describes a running gravity binary
type Executable struct {
	// Path specifies the path to the gravity binary
	Path string
	// WorkingDir specifies the working directory of the current process
	WorkingDir string
}

func must(exe *Executable, err error) *Executable {
	if err != nil {
		panic(err)
	}
	return exe
}
