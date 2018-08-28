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
