package utils

import (
	"os"

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
	gravityCommand = append(gravityCommand, "planet", "enter",
		"--", "--notty", command, "--")
	gravityCommand = append(gravityCommand, commandArgs...)

	return gravityCommand
}

// NewCurrentExecutable returns a new Executable for the currently running gravity binary
func NewCurrentExecutable() (*Executable, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Executable{
		Path: path,
	}, nil
}

// Executable abstracts the specified gravity binary
type Executable struct {
	// Path specifies the path to the gravity binary
	Path string
}

func must(exe *Executable, err error) *Executable {
	if err != nil {
		panic(err)
	}
	return exe
}
