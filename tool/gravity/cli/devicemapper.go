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

package cli

import (
	"fmt"
	"os"

	"github.com/gravitational/gravity/lib/devicemapper"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func devicemapperMount(disk string) error {
	entry := logrus.NewEntry(logrus.New())
	return devicemapper.Mount(disk, os.Stderr, entry)
}

func devicemapperUnmount() error {
	entry := logrus.NewEntry(logrus.New())
	return devicemapper.Unmount(os.Stderr, entry)
}

func devicemapperQuerySystemDirectory() error {
	dir, err := devicemapper.GetSystemDirectory()
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%v", dir)
	return nil
}
