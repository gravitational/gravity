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
	"bufio"
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// GetDevices queries the block devices on this system and returns the annonated
// list of available devices
func GetDevices() (devices []storage.Device, err error) {
	var out []byte
	out, err = exec.Command("lsblk", "--output=NAME,TYPE,SIZE,FSTYPE,PKNAME", "-P", "--bytes", "-I",
		strings.Join(supportedDeviceTypes, ",")).Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		err = trace.Wrap(err, "lsblk error=%v, stderr=%q, out=%q", exitErr, exitErr.Stderr, out)
		return nil, err
	} else if err != nil {
		return nil, trace.Wrap(err, "failed to list block devices: error=%v, output=%s", err, out)
	}

	devices, err = parseDevices(bytes.NewReader(out))
	if err != nil {
		err = trace.Wrap(err, "error parsing block device list: lsblk=%q : error=%v", out, err)
	}

	return devices, err
}

// StatDevice returns nil if the specified device exists and an error otherwise.
func StatDevice(path string) error {
	out, err := exec.Command("lsblk", path).Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return trace.Wrap(err, "lsblk error=%v, stderr=%q, out=%q", exitErr, exitErr.Stderr, out)
	} else if err != nil {
		return trace.Wrap(err, "failed to describe block device: error=%v, output=%s", err, out)
	}
	return nil
}

// Source: https://www.kernel.org/doc/Documentation/devices.txt
const (
	// deviceNumberSCSI identifies SCSI block devices
	deviceNumberSCSI = "8"

	// deviceNumberMetadisk identifies Metadisk block devices
	deviceNumberMetadisk = "9"

	// deviceNumberXen identifies Xen virtual block devices
	deviceNumberXen = "202"

	// deviceNumberExperimental identifies experimental block devices
	// TODO: experimental block device range is actually 240-254
	// 252 corresponds to a virsh device type
	deviceNumberExperimental = "252"

	// deviceNumberExperimental identifies "Local/Experimental" block devices, used with OpenStack storage
	deviceNumberLocalExperimental = "253"

	// deviceNumberNVMExpress identifies NVMExpress block devices
	deviceNumberNVMExpress = "259"
)

// supportedDeviceTypes lists all device types supported for discovery
var supportedDeviceTypes = []string{
	deviceNumberSCSI,
	deviceNumberMetadisk,
	deviceNumberXen,
	deviceNumberExperimental,
	deviceNumberLocalExperimental,
	deviceNumberNVMExpress,
}

// parseDevices interprets the specified reader as an output from:
//
// $ lsblk -P --output=NAME,TYPE,SIZE,FSTYPE,PKNAME --bytes -I 8
// NAME="sda" TYPE="disk" SIZE="68719476736" FSTYPE="" PKNAME=""
// NAME="sda1" TYPE="part" SIZE="524288000" FSTYPE="xfs" PKNAME="sda"
// NAME="sdb" TYPE="disk" SIZE="10737418240" FSTYPE="" PKNAME=""
// NAME="sdc" TYPE="disk" SIZE="10737418240" FSTYPE="" PKNAME=""
//
// Returns the list of devices annotated with name and type.
func parseDevices(r io.Reader) (devices []storage.Device, err error) {
	// columnName is the output column for device name
	const columnName = "NAME"
	// columnType is the output column for device type
	const columnType = "TYPE"
	// columnSize is the output column for device size
	const columnSize = "SIZE"
	// columnFilesystemType is the output column for mounted filesystem type
	const columnFilesystemType = "FSTYPE"
	// columnParentName is the output column for internal kernel parent device name
	const columnParentName = "PKNAME"

	s := bufio.NewScanner(r)
	s.Split(bufio.ScanLines)
	p := &parser{}
	deviceCache := make(map[string]storage.Device)
	for s.Scan() {
		columns := map[string]string{}
		line := strings.TrimSpace(s.Text())
		if len(line) == 0 {
			continue
		}
		p.scanner.Init(strings.NewReader(line))
		p.next()
		for p.token != scanner.EOF {
			attr := p.parseAttribute()
			if len(attr.value) > 0 {
				columns[attr.name] = attr.value
			}
		}
		if len(p.errors) > 0 {
			p.errors = append(p.errors, trace.Errorf("failed to parse %s", line))
			return nil, trace.NewAggregate(p.errors...)
		}
		deviceType := storage.DeviceType(columns[columnType])
		if deviceType == storage.DeviceDisk || deviceType == storage.DevicePartition {
			devicePath := filepath.Join("/dev", columns[columnName])
			parentName, hasParent := columns[columnParentName]

			if deviceType == storage.DevicePartition && hasParent {
				delete(deviceCache, filepath.Join("/dev", parentName))
			}

			if _, hasFilesystem := columns[columnFilesystemType]; hasFilesystem {
				continue
			}

			size, err := strconv.ParseUint(columns[columnSize], 10, 64)
			if err != nil {
				log.Infof("invalid size %v for device %v", columns[columnSize], devicePath)
			} else {
				size = size >> 20 // Mbytes
			}

			deviceCache[devicePath] = storage.Device{Name: storage.DeviceName(devicePath), Type: deviceType, SizeMB: size}
		}
	}
	for _, device := range deviceCache {
		devices = append(devices, device)
	}
	return devices, nil
}
