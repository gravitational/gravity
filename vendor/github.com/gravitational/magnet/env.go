package magnet

import (
	"context"
	"os"
	"strings"
	"sync"
)

var EnvVars map[string]EnvVar
var ImportEnvVars map[string]string

type EnvVar struct {
	Key     string
	Value   string
	Default string
	Short   string
	Long    string
	Secret  bool
}

func E(e EnvVar) string {
	if e.Key == "" {
		panic("Key shouldn't be empty")
	}

	if e.Secret && len(e.Default) > 0 {
		panic("Secrets shouldn't be embedded with defaults")
	}

	if EnvVars == nil {
		EnvVars = make(map[string]EnvVar)
		ImportEnvVars = make(map[string]string)
	}
	e.Value = os.Getenv(e.Key)

	EnvVars[e.Key] = e

	return GetEnv(e.Key)
}

func GetEnv(key string) string {
	importMakeEnv()

	if EnvVars == nil {
		EnvVars = make(map[string]EnvVar)
		ImportEnvVars = make(map[string]string)
	}

	if v, ok := EnvVars[key]; ok {
		if v.Value != "" {
			return v.Value
		}

		if v, ok := ImportEnvVars[key]; ok {
			return v
		}

		return v.Default
	}

	panic("Requested environment variable hasn't been registered")
}

var importOnce sync.Once

func importMakeEnv() {

	importOnce.Do(func() {
		out, err := Output(context.TODO(), "make", "magnet-vars")
		if err != nil {
			// suppress any errors
			return
		}

		lines := strings.Split(out, "\n")
		for _, line := range lines {
			cols := strings.SplitN(line, "=", 2)
			ImportEnvVars[cols[0]] = cols[1]
		}
	})
}
