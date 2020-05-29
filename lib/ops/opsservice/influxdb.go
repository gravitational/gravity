/*
Copyright 2020 Gravitational, Inc.

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

package opsservice

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/gravitational/trace"
)

func migrateInfluxDBData(server *ProvisionedServer, uid, gid string) (commands []Command, err error) {
	newDataDirectory := server.InGravity("monitoring", "influxdb")
	oldDataDirectory := "/var/lib/data/influxdb"

	if _, err = os.Stat(oldDataDirectory); os.IsNotExist(err) {
		// influxdb data is on another node
		return nil, nil
	}

	empty, err := isEmptyDir(newDataDirectory)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !empty {
		// directory already exist and not empty, probably data was migrated
		return nil, nil
	}

	commands = append(commands,
		Cmd(
			[]string{"cp", "-a", path.Join(oldDataDirectory, "/.", newDataDirectory)},
			"copying files from directory %v to %v", oldDataDirectory, newDataDirectory))
	commands = append(commands,
		Cmd(
			[]string{"chown", "-R", fmt.Sprintf("%v:%v", uid, gid), newDataDirectory},
			"setting ownership of %v to %v:%v", newDataDirectory, uid, gid))
	return commands, nil
}

func isEmptyDir(path string) (bool, error) {
	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}
	return len(entries) == 0, nil
}
