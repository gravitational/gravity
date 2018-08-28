package state

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// SetStateDir saves the provided directory stateDir as a local gravity state directory pointer
func SetStateDir(stateDir string) error {
	bytes, err := json.Marshal(stateLocator{StateDir: stateDir})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, path := range StateLocatorPaths {
		err := ioutil.WriteFile(path, bytes, defaults.SharedReadMask)
		if err == nil {
			log.Debugf("State dir locator written to %v.", path)
			return nil
		}
	}
	return trace.BadParameter(
		"could not write state dir locator to any of %v", StateLocatorPaths)
}

// GetStateDir returns local gravity state directory
func GetStateDir() (stateDir string, err error) {
	var bytes []byte
	for _, path := range StateLocatorPaths {
		bytes, err = ioutil.ReadFile(path)
		if err == nil {
			break
		}
	}
	if len(bytes) == 0 {
		return defaults.GravityDir, nil
	}
	var state stateLocator
	err = json.Unmarshal(bytes, &state)
	if err != nil {
		return "", trace.Wrap(err, "failed to unmarshal state locator")
	}
	log.Debugf("Found gravity state locator: %v.", state)
	if state.StateDir != "" {
		return state.StateDir, nil
	}
	return defaults.GravityDir, nil
}

type stateLocator struct {
	// StateDir is the gravity state directory
	StateDir string `json:"stateDir,omitempty"`
}

// Secret returns a full path to a secret
func Secret(baseDir, secretName string) string {
	return filepath.Join(baseDir, defaults.SecretsDir, secretName)
}

// Secret returns a secrets directory
func SecretDir(baseDir string) string {
	return filepath.Join(baseDir, defaults.SecretsDir)
}

// GravityUpdateDir returns full path to the update directory
func GravityUpdateDir(baseDir string) string {
	return filepath.Join(baseDir, defaults.SiteDir, defaults.UpdateDir)
}

// GravityRPCAgentDir returns full path to the RPC agent directory
func GravityRPCAgentDir(baseDir string) string {
	return filepath.Join(baseDir, defaults.SiteDir, defaults.UpdateDir, defaults.AgentDir)
}

// ShareDir returns full path to the planet share directory
func ShareDir(baseDir string) string {
	return filepath.Join(baseDir, defaults.PlanetDir, defaults.ShareDir)
}

// RegistryDir returns full path to the planet docker registry directory
func RegistryDir(baseDir string) string {
	return filepath.Join(baseDir, defaults.PlanetDir, defaults.StateRegistryDir)
}

// LogDir returns full path to the planet log directory
func LogDir(baseDir string, suffixes ...string) string {
	elems := []string{baseDir, defaults.PlanetDir, defaults.LogDir}
	return filepath.Join(append(elems, suffixes...)...)
}

var (
	// StateLocatorPaths is a list of locations where gravity state directory pointer is written
	StateLocatorPaths = []string{
		filepath.Join(defaults.EtcDir, defaults.GravityConfigFilename),
		filepath.Join(defaults.EtcWritableDir, defaults.GravityConfigFilename),
		filepath.Join(defaults.WritableDir, defaults.GravityConfigFilename),
	}

	// GravityBinPaths is a list of possible gravity binary locations on host
	GravityBinPaths = []string{
		defaults.GravityBin,
		defaults.GravityBinAlternate,
	}
)
