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

package storage

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewSystemInfo creates a new instance of system information
// from the provided spec
func NewSystemInfo(spec SystemSpecV2) *SystemV2 {
	return &SystemV2{
		Kind:    KindSystemInfo,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      "systeminfo",
			Namespace: teledefaults.Namespace,
		},
		Spec: spec,
	}
}

// System describes a system
type System interface {
	teleservices.Resource

	// GetHostname returns the system hostname
	GetHostname() string
	// GetNetworkInterfaces returns the list of network interfaces
	GetNetworkInterfaces() map[string]NetworkInterface
	// GetFilesystems returns the mounted files systems
	GetFilesystems() []Filesystem
	// GetFilesystemStats returns the mounted files systems
	GetFilesystemStats() FilesystemStats
	// GetMemory returns the RAM configuration
	GetMemory() Memory
	// GetSwap returns the swap configuration
	GetSwap() Swap
	// GetNumCPU returns the number of CPUs
	GetNumCPU() uint
	// GetProcesses returns the list of running processes
	GetProcesses() []Process
	// GetDevices returns the list of unallocated devices
	GetDevices() Devices
	// GetSystemPackages returns the list of installed system packages
	GetSystemPackages() []SystemPackage
	// GetOS identifies the host operating system or distribution
	GetOS() OSInfo
	// GetUser returns the information about the user the agent is running under
	GetUser() OSUser
}

// UnmarshalSystemInfo unmarshals system info from JSON specified with data
func UnmarshalSystemInfo(data []byte) (*SystemV2, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("empty data payload")
	}

	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var hdr teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &hdr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch hdr.Version {
	case teleservices.V2:
		var info SystemV2
		err := teleutils.UnmarshalWithSchema(GetSystemInfoSchema(), &info, jsonData)
		if err != nil {
			log.WithFields(log.Fields{
				log.ErrorKey: err,
				"source":     string(jsonData),
			}).Warn("Failed to validate JSON against schema.")
			return nil, trace.BadParameter(err.Error())
		}
		err = info.Metadata.CheckAndSetDefaults()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &info, nil
	}
	return nil, trace.BadParameter(
		"%v resource version %q is not supported.", KindSystemInfo, hdr.Version)
}

// MarshalSystemInfo marshals the specified system info object to JSON
func MarshalSystemInfo(info System, opts ...teleservices.MarshalOption) ([]byte, error) {
	data, err := json.Marshal(info)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// GetHostname returns the system hostname
func (r *SystemV2) GetHostname() string {
	return r.Spec.Hostname
}

// GetNetworkInterfaces returns the list of network interfaces
func (r *SystemV2) GetNetworkInterfaces() map[string]NetworkInterface {
	return r.Spec.NetworkInterfaces
}

// GetFilesystems returns the mounted files systems
func (r *SystemV2) GetFilesystems() []Filesystem {
	return r.Spec.Filesystems
}

// GetFilesystemStats returns the mounted files systems
func (r *SystemV2) GetFilesystemStats() FilesystemStats {
	return r.Spec.FilesystemStats
}

// GetMemory returns the RAM configuration
func (r *SystemV2) GetMemory() Memory {
	return r.Spec.Memory
}

// GetSwap returns the swap configuration
func (r *SystemV2) GetSwap() Swap {
	return r.Spec.Swap
}

// GetNumCPU returns the number of CPUs
func (r *SystemV2) GetNumCPU() uint {
	return r.Spec.NumCPU
}

// GetProcesses returns the list of running processes
func (r *SystemV2) GetProcesses() []Process {
	return r.Spec.Processes
}

// GetDevices returns the list of unallocated devices
func (r *SystemV2) GetDevices() Devices {
	return Devices(r.Spec.Devices)
}

// GetSystemPackages returns the list of installed system packages
func (r *SystemV2) GetSystemPackages() []SystemPackage {
	return r.Spec.SystemPackages
}

// GetOS identifies the host operating system or distribution
func (r *SystemV2) GetOS() OSInfo {
	return r.Spec.OS
}

// GetUser returns the information about the user the agent is running under
func (r *SystemV2) GetUser() OSUser {
	return r.Spec.User
}

// SystemV2 describes a system
type SystemV2 struct {
	// Kind is resource kind, "systeminfo"
	Kind string `json:"kind"`
	// Version is the resource version
	Version string `json:"version"`
	// Metadata is resource metadata
	teleservices.Metadata `json:"metadata"`
	// Spec is the system information spec
	Spec SystemSpecV2 `json:"spec"`
}

// SystemSpecV2 represents a set of facts about a system
type SystemSpecV2 struct {
	// Hostname specifies the hostname
	Hostname string `json:"hostname"`
	// NetworkInterfaces lists all network interfaces
	NetworkInterfaces map[string]NetworkInterface `json:"interfaces"`
	// Filesystem returns information about filesystem usage
	Filesystems []Filesystem `json:"filesystem"`
	// FilesystemStats returns information about filesystem usage per directory
	FilesystemStats FilesystemStats `json:"filesystem_stats"`
	// Memory contains information about system memory
	Memory Memory `json:"memory"`
	// Swap contains info about system's swap capacity
	Swap Swap `json:"swap"`
	// NumCPU specifies the CPU count
	NumCPU uint `json:"cpus"`
	// Processes lists running processes
	Processes []Process `json:"processes"`
	// Devices lists the disks/partitions
	Devices Devices `json:"devices"`
	// SystemPackages lists installed system packages.
	// Packages are queried per distribution.
	// Only packages required for operation are listed
	SystemPackages []SystemPackage `json:"system_packages"`
	// OS identifies the host operating system
	OS OSInfo `json:"os"`
	// LVMSystemDirectory specifies the location of the LVM system directory if the
	// docker storage driver is devicemapper, empty otherwise
	// DEPRECATED
	LVMSystemDirectory string `json:"lvm_system_dir"`
	// User specifies the agent's user identity
	User OSUser `json:"user"`
}

// String returns a textual representation of this system info
func (r SystemV2) String() string {
	var ifaces []string
	for name, iface := range r.Spec.NetworkInterfaces {
		ifaces = append(ifaces, fmt.Sprintf("%v=%v", name, iface.IPv4))
	}
	return fmt.Sprintf("sysinfo(hostname=%v, interfaces=%v, cpus=%v, ramMB=%v, OS=%v, user=%v)",
		r.Spec.Hostname,
		strings.Join(ifaces, ","),
		r.Spec.NumCPU,
		r.Spec.Memory.Total/1000/1000,
		r.Spec.OS,
		r.Spec.User,
	)
}

// GetSystemInfoSchema returns system information schema for version V2
func GetSystemInfoSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		SystemSpecV2Schema, "")
}

// SystemSpecV2Schema is JSON schema for host system information
const SystemSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["hostname", "interfaces", "filesystem", "filesystem_stats",
      "memory", "swap", "cpus", "processes", "devices", "system_packages",
      "os", "lvm_system_dir", "user"],
  "properties": {
    "hostname": {"type": "string"},
    "interfaces": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "required": ["ipv4_addr", "name"],
        "properties": {
          "ipv4_addr": {"type": "string"},
          "name": {"type": "string"}
        }
      }
    },
    "filesystem": {
      "type": ["array", "null"],
      "additionalProperties": false,
      "required": ["dir_name", "systype_name"],
      "properties": {
        "dir_name": {"type": "string"},
        "systype_name": {"type": "string"}
      }
    },
    "filesystem_stats": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "required": ["total", "free"],
        "additionalProperties": false,
        "properties": {
          "total": {"type": "integer"},
          "free": {"type": "integer"}
        }
      }
    },
    "memory": {
      "type": "object",
      "required": ["total", "free", "actual_free"],
      "additionalProperties": false,
      "properties": {
        "total": {"type": "integer"},
        "free": {"type": "integer"},
        "actual_free": {"type": "integer"}
      }
    },
    "swap": {
      "type": "object",
      "required": ["total", "free"],
      "additionalProperties": false,
      "properties": {
        "total": {"type": "integer"},
        "free": {"type": "integer"}
      }
    },
    "cpus": {"type": "integer"},
    "processes": {
      "type": ["array", "null"],
      "required": ["name", "pid"],
      "additionalProperties": false,
      "properties": {
        "name": {"type": "string"},
        "pid": {"type": "integer"}
      }
    },
    "devices": {
      "type": ["array", "null"],
      "items": {
        "type": "object",
        "required": ["name", "type", "size_mb"],
        "additionalProperties": false,
        "properties": {
          "name": {"type": "string"},
          "type": {"type": "string"},
          "size_mb": {"type": "integer"}
        }
      }
    },
    "system_packages": {
      "type": ["array", "null"],
      "items": {
        "type": "object",
        "required": ["name", "version", "error"],
        "additionalProperties": false,
        "properties": {
          "name": {"type": "string"},
          "version": {"type": "string"},
          "error": {"type": "string"}
        }
      }
    },
    "os": {
      "type": "object",
      "required": ["name", "version"],
      "additionalProperties": false,
      "properties": {
        "name": {"type": "string"},
        "like": {"type": "array", "items": {"type": "string"}},
        "version": {"type": "string"}
      }
    },
    "lvm_system_dir": {"type": "string"},
    "user": {
      "type": "object",
      "required": ["name", "uid", "gid"],
      "additionalProperties": false,
      "properties": {
        "name": {"type": "string"},
        "uid": {"type": "string"},
        "gid": {"type": "string"}
      }
    }
  }
}`

// Filesystem describes a mounted file system
type Filesystem struct {
	// DirName specifies the name of the directory where the file system is mounted
	DirName string `json:"dir_name"`
	// Type is the file system type
	Type string `json:"type"`
}

// FilesystemStats maps a directory name to usage information
type FilesystemStats map[string]FilesystemUsage

// FilesystemUsage describes usage for a mounted file system
type FilesystemUsage struct {
	// TotalKB is the amount of space on a file system, in kilobytes (KB)
	TotalKB uint64 `json:"total"`
	// FreeKB is the amount of free space on a file system, in kilobytes (KB)
	FreeKB uint64 `json:"free"`
}

// Memory describes RAM parameters on a system
type Memory struct {
	// Total is the amount of physical RAM, in kilobytes (kB)
	Total uint64 `json:"total"`
	// Free is the amount of physical RAM left unused, in kilobytes (kB)
	Free uint64 `json:"free"`
	// ActualFree is the amount of free RAM
	// (accounting for kernel-allocated memory), in kilobytes
	ActualFree uint64 `json:"actual_free"`
}

// Swap describes swapping configuration
type Swap struct {
	// Total is total amount of swap, in kilobytes
	Total uint64 `json:"total"`
	// Free is total amount of swap free, in kilobytes
	Free uint64 `json:"free"`
}

// NetworkInterface represents a network interface
type NetworkInterface struct {
	// IPv4 address assigned to the interface
	IPv4 string `json:"ipv4_addr"`
	// Name is the interface name
	Name string `json:"name"`
}

// Process represents a running process
type Process struct {
	// Name is the process executable name
	Name string `json:"name"`
	// PID is the process ID
	PID int `json:"pid"`
}

// OSInfo describes an operating system using several attributes like operating system ID
// and a version number
type OSInfo struct {
	// ID defines the system with a single word ID: `ubuntu` or `rhel`
	ID string `json:"name"`
	// Like defines the system as being similar to given ones: `debian` or `rhel fedora`
	Like []string `json:"like,omitempty"`
	// Version defines the numeric version of the system: `7.2`
	Version string `json:"version"`
}

// OSUser describes a user on host.
type OSUser struct {
	// Name of the user. Not empty if this describes an existing user
	Name string `json:"name"`
	// UID specifies the user ID
	UID string `json:"uid"`
	// GID specifies the group ID
	GID string `json:"gid"`
}

// ResolvConf describes the system resolv.conf configuration
type ResolvConf struct {
	// Servers - Name server IP addresses
	Servers []string
	// Domain - Local domain name
	Domain string
	// Search list for host-name lookup
	Search []string
	// Ndots is the number of dots in name to trigger absolute lookup
	Ndots int
	// Timeout is the number of seconds the resolver will wait for a response from the remote server
	Timeout int
	// Attempts is the number of times the resolver will send queries before giving up
	Attempts int
	// Rotate sets round robin selection of nameservers
	Rotate bool
	// UnknownOpt indicates whether we received any unknown options
	UnknownOpt bool
	// Lookup is OpenBSD top-level database "lookup" order
	Lookup []string
}

func DefaultOSUser() OSUser {
	return OSUser{
		defaults.ServiceUser,
		defaults.ServiceUserID,
		defaults.ServiceGroupID,
	}
}

// IsEmpty determines if this user is empty.
// User is not empty if it has a name.
func (r OSUser) IsEmpty() bool {
	return r.UID == ""
}

// SystemPackage describes a package on a Linux system
type SystemPackage struct {
	// Name identifies the package by name
	Name string `json:"name"`
	// Version describes the version of the installed package.
	// It will be empty if no such package is installed
	Version string `json:"version"`
	// Error describes an error querying for the package
	Error string `json:"error"`
}
