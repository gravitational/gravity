package report

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/utils"

	log "github.com/sirupsen/logrus"
	"github.com/gravitational/trace"
)

// Collect fetches shell histories for all users from passwd.
// Collect implements Collector
func (r bashHistoryCollector) Collect(reportWriter Writer, runner utils.CommandRunner) error {
	log.Debug("collecting bash histories")
	passwd, err := utils.GetPasswd()
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer passwd.Close()

	users, err := utils.ParsePasswd(passwd)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	for _, user := range users {
		w, err := reportWriter(fmt.Sprintf("bash_history-%v", user.Name))
		if err != nil {
			log.Warningf("failed to create writer for bash history for user %q", user.Name)
			continue
		}
		defer w.Close()

		path := filepath.Join(user.Home, bashHistoryFileName)
		f, err := os.Open(path)
		if err != nil {
			log.Warningf("failed to fetch bash history for user %q: %v",
				user.Name, trace.ConvertSystemError(err))
			continue
		}
		defer f.Close()

		_, err = io.Copy(w, f)
		if err != nil {
			log.Warningf("failed to read bash history file %q (%q): %v",
				path, user.Name, trace.ConvertSystemError(err))
		}
	}

	return nil
}

type bashHistoryCollector struct{}

const bashHistoryFileName = ".bash_history"
