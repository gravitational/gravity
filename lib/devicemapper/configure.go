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

package devicemapper

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// PoolName is the name of the pool device configured for docker
const PoolName = "docker-thinpool"

// Mount creates devicemapper environment for docker using disk as a media
// for the physical volume
func Mount(disk string, out io.Writer, entry log.FieldLogger) (err error) {
	config := &config{
		FieldLogger: entry,
		disk:        disk,
		out:         out,
	}
	if err = config.createPhysicalVolume(); err != nil {
		return trace.Wrap(err)
	}
	if err = config.createVolumeGroup(); err != nil {
		return trace.Wrap(err)
	}
	if err = config.setProfileConfiguration(&profileContext{
		Threshold: defaults.DevicemapperAutoextendThreshold,
		Percent:   defaults.DevicemapperAutoextendStep,
	}); err != nil {
		return trace.Wrap(err)
	}
	if err = config.createThinPool(); err != nil {
		return trace.Wrap(err)
	}
	if err = config.applyVolumeProfile(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Unmount removes docker devicemapper environment
func Unmount(out io.Writer, logger log.FieldLogger) (err error) {
	disk, err := queryPhysicalVolume(logger)
	if err != nil {
		return trace.Wrap(err)
	}
	if disk == "" {
		logger.Info("No physical volumes found.")
		return nil
	}
	logger.Infof("Found physical volume on disk %v.", disk)
	config := &config{
		FieldLogger: logger,
		disk:        disk,
		out:         out,
	}
	if err = config.removeLingeringDevices(); err != nil {
		return trace.Wrap(err)
	}
	if err = config.removeLogicalVolume(); err != nil {
		return trace.Wrap(err)
	}
	if err = config.removeVolumeGroup(); err != nil {
		return trace.Wrap(err)
	}
	if err = config.removePhysicalVolume(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func queryPhysicalVolume(logger log.FieldLogger) (disk string, err error) {
	logger.Debug("Query physical volume information.")

	out := bytes.Buffer{}
	cmd := exec.Command("vgs", "-o", "pv_name", "--noheadings", "-S", fmt.Sprintf("vg_name=%v", volumeGroup))
	if err = utils.ExecL(cmd, &out, logger); err != nil {
		return "", trace.Wrap(err, "failed to query physical volume information")
	}
	return strings.TrimSpace(out.String()), nil
}

func (r *config) createPhysicalVolume() (err error) {
	r.Debugf("create physical volume on disk %v", r.disk)

	cmd := exec.Command("pvcreate", r.disk)
	if err = r.exec(cmd); err != nil {
		return trace.Wrap(err, "failed to create physical volume on disk %v", r.disk)
	}
	return nil
}

func (r *config) removePhysicalVolume() (err error) {
	r.Debugf("remove physical volume on disk %v", r.disk)

	query := exec.Command("pvs", "--noheadings", "-o", "pv_name", r.disk)
	if err = r.exec(query); err == nil {
		cmd := exec.Command("pvremove", r.disk)
		if err = r.exec(cmd); err != nil {
			return trace.Wrap(err, "failed to remove physical volume on disk %v", r.disk)
		}
	}

	return nil
}

func (r *config) createVolumeGroup() (err error) {
	r.Debugf("create volume group %v on disk %v", volumeGroup, r.disk)

	cmd := exec.Command("vgcreate", volumeGroup, r.disk)
	if err = r.exec(cmd); err != nil {
		return trace.Wrap(err, "failed to create volume group %v on disk %v", volumeGroup, r.disk)
	}
	return nil
}

func (r *config) removeVolumeGroup() (err error) {
	r.Debugf("remove volume group %v on disk %v", volumeGroup, r.disk)

	query := exec.Command("vgs", "--noheadings", "-o", "vg_name", volumeGroup)
	if err = r.exec(query); err == nil {
		cmd := exec.Command("vgremove", volumeGroup)
		if err = r.exec(cmd); err != nil {
			return trace.Wrap(err, "failed to remove volume group %v on disk %v", volumeGroup, r.disk)
		}
	}

	return nil
}

func (r *config) removeLogicalVolume() (err error) {
	volume := fmt.Sprintf("%v/%v", volumeGroup, logicalVolume)
	r.Debugf("remove logical volume %v", volume)

	query := exec.Command("lvs", "--noheadings", "-o", "lv_name", volume)
	if err = r.exec(query); err == nil {
		cmd := exec.Command("lvremove", "-f", volume)
		if err = r.exec(cmd); err != nil {
			return trace.Wrap(err, "failed to remove volume %v", volume)
		}
	}
	return nil
}

func (r *config) createThinPool() (err error) {
	r.Debugf("create thin pool")

	// TODO: customize volume group reservation size for data/meta sections
	createPool := func(pool, reservation string) error {
		cmd := exec.Command("lvcreate", "--wipesignatures", "y", "-n",
			pool, volumeGroup, "-l", reservation)
		if err = r.exec(cmd); err != nil {
			return trace.Wrap(err, "failed to create pool %v for group %v", pool, volumeGroup)
		}
		return nil
	}
	r.Debugf("create logical volume for data")
	if err = createPool("thinpool", "95%VG"); err != nil {
		return trace.Wrap(err)
	}
	// reserve 4% for auto-expansion of data/meta sections
	r.Debugf("create logical volume for metadata")
	if err = createPool("thinpoolmeta", "1%VG"); err != nil {
		return trace.Wrap(err)
	}

	pool := fmt.Sprintf("%v/thinpool", volumeGroup)
	poolMeta := fmt.Sprintf("%v/thinpoolmeta", volumeGroup)
	r.Debugf("convert pool `%v/%v` to a thin pool", pool, poolMeta)
	cmd := exec.Command("lvconvert", "-y", "--zero", "n", "-c", "512K",
		"--thinpool", pool, "--poolmetadata", poolMeta)
	if err = r.exec(cmd); err != nil {
		return trace.Wrap(err, "failed to convert pool `%v/%v` to a thin pool", pool, poolMeta)
	}
	return nil
}

func (r *config) setProfileConfiguration(ctx *profileContext) error {
	r.Debugf("save metadata profile configuration")

	err := os.MkdirAll(filepath.Dir(profilePath), defaults.SharedDirMask)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	file, err := os.Create(profilePath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer file.Close()

	err = profileTemplate.Execute(file, ctx)
	return trace.Wrap(err)
}

func (r *config) applyVolumeProfile() (err error) {
	pool := fmt.Sprintf("%v/thinpool", volumeGroup)
	r.Debugf("apply configuration profile %v to logical volume metadata", PoolName)

	cmd := exec.Command("lvchange", "--metadataprofile", PoolName, pool)
	if err = r.exec(cmd); err != nil {
		return trace.Wrap(err, "failed to apply the volume profile for pool %v", pool)
	}
	return nil
}

func (r *config) removeLingeringDevices() error {
	unmount := func(name string) error {
		return r.exec(exec.Command("umount", name))
	}
	remove := func(name string) error {
		return r.exec(exec.Command("dmsetup", "remove", name))
	}

	matches, err := filepath.Glob(dockerDeviceMask)
	if err != nil {
		return trace.Wrap(err, "failed to enumerate docker devicemapper devices")
	}
	for _, match := range matches {
		if err := unmount(match); err != nil {
			// Failure to unmount is not fatal
			r.Warningf("failed to unmount %v: %v", match, err)
		}
		if err := remove(match); err != nil {
			return trace.Wrap(err, "failed to remove docker device %v", match)
		}
	}
	return nil
}

func (r *config) exec(cmd *exec.Cmd) error {
	return utils.ExecL(cmd, r.out, r.FieldLogger)
}

// volumeGroup sets the name of the volume group to create
const volumeGroup = "docker"

// logicalVolume names the logical volume created for docker
const logicalVolume = "thinpool"

// dockerDeviceMask defines the file name pattern to look for docker devicemapper
// devices
const dockerDeviceMask = "/dev/mapper/docker-*"

// dockerProfilePath defines the metadata profile settings file for docker
var profilePath = filepath.Join("/etc", "lvm", "profile", fmt.Sprintf("%v.profile", PoolName))

var profileTemplate = template.Must(template.New("profile").Parse(`
  activation {
    thin_pool_autoextend_threshold={{ .Threshold }}
    thin_pool_autoextend_percent={{ .Percent }}
  }
`))

type config struct {
	log.FieldLogger

	// disk defines a disk or partition to use for devicemapper configuration
	disk string
	out  io.Writer
}

type profileContext struct {
	// Threshold is the percentage of space used before LVM automatically
	// attempts to extend the available space (100=disabled)
	Threshold int
	// Percent defines the extension step in percent
	Percent int
}
