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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// Getenv returns the map of name->value pairs that captures the specified list
// of environment variables. Only variables with a value are captured
func Getenv(envs ...string) (environ map[string]string) {
	environ = make(map[string]string)
	for _, env := range envs {
		if value, ok := os.LookupEnv(env); ok {
			environ[env] = value
		}
	}
	return environ
}

// GetenvWithDefault returns the value of the environment variable given
// with name or defaultValue if the variable does not exist
func GetenvWithDefault(name, defaultValue string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return defaultValue
}

// GetenvsByPrefix returns environment variables with names matching specified prefix.
func GetenvsByPrefix(prefix string) (environ map[string]string) {
	environ = make(map[string]string)
	for _, env := range os.Environ() {
		name := strings.SplitN(env, "=", 2)[0]
		if strings.HasPrefix(name, prefix) {
			environ[name] = os.Getenv(name)
		}
	}
	return environ
}

// GetenvInt returns the specified environment variable value parsed as an integer.
func GetenvInt(name string) (int, error) {
	valueS, ok := os.LookupEnv(name)
	if !ok {
		return 0, trace.NotFound("environment variable %v not set", name)
	}
	valueI, err := strconv.Atoi(valueS)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return valueI, nil
}

// runningInsideContainer specifies if this process is executing inside
// planet container
var runningInsideContainer bool
var contextCheck sync.Once
