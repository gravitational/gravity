package magnet

import "os"

var EnvVars map[string]EnvVar

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
	}
	e.Value = os.Getenv(e.Key)

	EnvVars[e.Key] = e

	return GetEnv(e.Key)
}

func GetEnv(key string) string {
	if v, ok := EnvVars[key]; ok {
		if v.Value != "" {
			return v.Value
		}
		return v.Default
	}

	panic("Requested environment variable hasn't been registered")
}
