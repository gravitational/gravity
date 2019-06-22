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

package report

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Collect fetches shell histories for all users from passwd.
// Collect implements Collector
func (r bashHistoryCollector) Collect(ctx context.Context, reportWriter FileWriter, runner utils.CommandRunner) error {
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
		w, err := reportWriter.NewWriter(fmt.Sprintf("bash_history-%v", user.Name))
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
