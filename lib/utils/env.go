package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/trace"
)

// ReadEnv reads the file at the specified path as a file containing environment
// variables (e.g. /etc/environment)
func ReadEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()
	env := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		kv := strings.SplitN(scanner.Text(), "=", 2)
		if len(kv) != 2 {
			continue // skip bad env vars
		}
		env[kv[0]] = kv[1]
	}
	return env, trace.Wrap(scanner.Err())
}

// WriteEnv writes the provided env as an environment variables file
// at the specified path
func WriteEnv(path string, env map[string]string) error {
	err := os.MkdirAll(filepath.Dir(path), defaults.SharedDirMask)
	if err != nil {
		return trace.Wrap(err)
	}
	file, err := os.Create(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	for name, value := range env {
		_, err = fmt.Fprintf(file, "%v=%v\n", name, value)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DetectPlanetEnvironment detects if the process is executed inside
// the container
func DetectPlanetEnvironment() {
	contextCheck.Do(func() {
		_, err := StatFile(defaults.ContainerEnvironmentFile)
		runningInsideContainer = err == nil
	})
}

// CheckInPlanet returns whether the process was started inside
// the container
func CheckInPlanet() bool {
	return runningInsideContainer
}

// runningInsideContainer specifies if this process is executing inside
// planet container
var runningInsideContainer bool
var contextCheck sync.Once
