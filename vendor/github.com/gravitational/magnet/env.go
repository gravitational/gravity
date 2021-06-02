package magnet

import (
	"fmt"
	"os"
)

// EnvVar represents a configuration with optional defaults
// obtained from environment
type EnvVar struct {
	Key     string
	Value   string
	Default string
	Short   string
	Long    string
	Secret  bool
}

// E defines a new environment variable specified with e.
// Returns the current value of the variable with precedence
// given to previously imported environment variables.
// If the variable was not previously imported and no value
// has been specified, the default is returned
func (m *Magnet) E(e EnvVar) string {
	if e.Key == "" {
		panic("Key shouldn't be empty")
	}

	if e.Secret && len(e.Default) > 0 {
		panic("Secrets shouldn't be embedded with defaults")
	}

	if v, ok := m.ImportEnv[e.Key]; ok {
		e.Value = v
	} else {
		e.Value = os.Getenv(e.Key)
	}
	m.env[e.Key] = e

	return m.MustGetEnv(e.Key)
}

// MustGetEnv returns the value of the environment variable given with key.
// The variable is assumed to have been registered either with E or
// imported from existing environment - otherwise the function will panic.
// For non-panicking version use GetEnv
func (m *Magnet) MustGetEnv(key string) (value string) {
	if v, ok := m.GetEnv(key); ok {
		return v
	}
	panic(fmt.Sprintf("Requested environment variable %q hasn't been registered", key))
}

// GetEnv returns the value of the environment variable given with key.
// The variable is assumed to have been registered either with E or
// imported from existing environment
func (m *Magnet) GetEnv(key string) (value string, exists bool) {
	var v EnvVar
	if v, exists = m.env[key]; !exists {
		return "", false
	}
	if v.Value != "" {
		return v.Value, true
	}
	return v.Default, true
}

// Env returns the complete environment
func (m *Magnet) Env() map[string]EnvVar {
	return m.env
}
