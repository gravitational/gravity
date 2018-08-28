package cli

import (
	"net/url"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// BuildEnv returns a new instance of local environment for tele build
func (t *Application) BuildEnv() (*localenv.LocalEnvironment, error) {
	var cacheDir string
	var err error
	if *t.StateDir != "" {
		// local state directory is used to build installer from
		// locally available packages
		cacheDir = *t.StateDir
	} else {
		cacheDir, err = ensureCacheDir(*t.BuildCmd.Repository)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return localenv.NewLocalEnvironment(
		localenv.LocalEnvironmentArgs{
			StateDir:         cacheDir,
			LocalKeyStoreDir: *t.StateDir,
			Insecure:         *t.Insecure,
		})
}

// ensureCacheDir makes sure a local cache directory for the provided Ops Center
// exists
func ensureCacheDir(opsURL string) (string, error) {
	u, err := url.Parse(opsURL)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// cache directory is ~/.gravity/cache/<opscenter>/
	dir, err := utils.EnsureLocalPath("", defaults.LocalCacheDir, u.Host)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return dir, nil
}
