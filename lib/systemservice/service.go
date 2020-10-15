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

package systemservice

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

const (
	// ServiceStatusActivating indicates that service is activating
	ServiceStatusActivating = "activating"
	// ServiceStatusFailed means taht service has failed
	ServiceStatusFailed = "failed"
	// ServiceStatusActive means that service is active
	ServiceStatusActive = "active"
	// ServiceStatusInactive indicates that service is not running
	// Corresponds to exit code 3
	ServiceStatusInactive = "inactive"
	// ServiceStatusUnknown indicates that service does not exist or the status
	// could not be determined - depending on the command
	ServiceStatusUnknown = "unknown"
)

// FullServiceName returns the full service name (incl. the suffix).
// It will append the service suffix if necessary
func FullServiceName(serviceName string) (nameWithSuffix string) {
	if filepath.Ext(serviceName) != ServiceSuffix {
		return fmt.Sprint(serviceName, ServiceSuffix)
	}
	return serviceName
}

// ServiceSuffix specifies the suffix of the systemd service file
const ServiceSuffix = ".service"

// IsKnownStatus returns whether passed service status is a known status
func IsKnownStatus(s string) bool {
	switch s {
	case ServiceStatusActivating, ServiceStatusFailed, ServiceStatusActive:
		return true
	}
	return false
}

// NewServiceRequest describes a request to create a systemd service
type NewServiceRequest struct {
	// ServiceSpec defines the service
	ServiceSpec
	// Name is the service name.
	// It can be the absolute path to the unit file if the file is located
	// in a non-standard location
	Name string `json:"Name"`
	// NoBlock means we won't block and wait until service starts
	NoBlock bool `json:"-"`
	// ReloadConfiguration forces a daemon-reload after writing the service file
	ReloadConfiguration bool `json:"-"`
}

// NewMountServiceRequest describes a request to create a new systemd mount service
type NewMountServiceRequest struct {
	// ServiceSpec defines the mount service
	ServiceSpec MountServiceSpec
	// Name is a service name, e.g. temp.service
	Name string `json:"Name"`
	// NoBlock means we won't block and wait until service starts
	NoBlock bool `json:"-"`
}

// UninstallServiceRequest describes a request to uninstall a service
type UninstallServiceRequest struct {
	// Name identifies the service
	Name string
}

// DisableServiceRequest describes a request to disable a service
type DisableServiceRequest struct {
	// Name identifies the service
	Name string
	// Now specifies whether the service is also stopped
	Now bool
}

// NewPackageServiceRequest specifies parameters needed to create a new service
// that is using gravity package manager
type NewPackageServiceRequest struct {
	ServiceSpec
	// GravityPath is a path to gravity executable
	GravityPath string `json:"-"`
	// Package is a package holding a command to execute
	Package loc.Locator `json:"-"`
	// ConfigPackage is a package with configuration
	ConfigPackage loc.Locator `json:"-"`
	// NoBlock means async operation
	NoBlock bool `json:"-"`
}

// ServiceSpec is a generic service specification
type ServiceSpec struct {
	// Dependencies defines dependencies to other services
	Dependencies Dependencies `json:"Dependencies"`
	// StartCommand defines the command to execute when the service starts
	StartCommand string `json:"StartCommand"`
	// StartPreCommand defines the commands to execute before the service starts
	StartPreCommands []string `json:"StartPreCommands,omitempty"`
	// StartPostCommand defines the command to execute after the service starts
	StartPostCommand string `json:"StartPostCommand"`
	// StopCommand defines the command to execute when the service stops
	StopCommand string `json:"StopCommand"`
	// StopPostCommand defines the command to execute after the service stops
	StopPostCommand string `json:"StopPostCommand"`
	// Timeout is a timeout in seconds
	Timeout int `json:"Timeout"`
	// Type is a service type
	Type string `json:"Type"`
	// User is a user name owning the process
	User string `json:"User"`
	// LimitNoFile sets ulimits for this process
	LimitNoFile int `json:"LimitNOFILE"`
	// Restart sets restart policy
	Restart string `json:"Restart"`
	// RestartSec is a period between restarts
	RestartSec int `json:"RestartSec"`
	// KillMode is a systemd kill mode, 'none' by default
	KillMode string `json:"KillMode"`
	// KillSignal specifies the signal to use. Defaults to SIGTERM.
	// See https://www.freedesktop.org/software/systemd/man/systemd.kill.html
	KillSignal string `json:"KillSignal"`
	// WantedBy sets up basic target this service is wanted by,
	// changes install section
	WantedBy string `json:"WantedBy"`
	// RemainAfterExit tells service to remain after the process has exited
	RemainAfterExit bool `json:"RemainAfterExit"`
	// Environment is environment variables to set for the service
	Environment map[string]string `json:"Environment"`
	// TasksMax specifies the maximum number of tasks that may be created in the unit
	TasksMax string `json:"TasksMax"`
	// TimeoutStopSec specifies the stop timeout.
	// The value is either a unitless value in seconds or a string such as "5min 20s" or "infinity"
	// to disable timeout logic.
	TimeoutStopSec string `json:"TimeoutStopSec"`
	// ConditionPathExists specifies start condition for the service based on existence
	// of the specified file. Can be negated by prefixing the path with "!"
	ConditionPathExists string `json:"ConditionPathExists"`
	// RestartPreventExitStatus lists exit status definitions that, when returned by the main service
	// process, will prevent automatic service restarts.
	// See https://www.freedesktop.org/software/systemd/man/systemd.service.html#RestartPreventExitStatus=
	RestartPreventExitStatus string `json:"RestartPreventExitStatus"`
	// SuccessExitStatus lists exit codes to be considered successful termination.
	// See https://www.freedesktop.org/software/systemd/man/systemd.service.html#SuccessExitStatus=
	SuccessExitStatus string `json:"SuccessExitStatus"`
	// WorkingDirectory sets the working directory for executed processes.
	// See https://www.freedesktop.org/software/systemd/man/systemd.exec.html#Paths
	WorkingDirectory string `json:"WorkingDirectory"`
}

// MountServiceSpec describes specification for a systemd mount service
type MountServiceSpec struct {
	// What specifies defines the absolute path of a device node, file or other resource to mount
	What string `json:"where"`
	// Where specifies the absolute path of a directory for the mount point
	Where string `json:"what"`
	// Type specifies the file system type.
	// This setting is optional
	Type string `json:"type"`
	// Options lists mount options to use when mounting
	// This setting is optional
	Options []string `json:"options,omitempty"`
	// TimeoutSec configures the time to wait for the mount command to finish.
	// Takes a unit-less value in seconds, or a time span value such as "5min 20s".
	// Pass "0" to disable the timeout logic.
	// This setting is optional and the default is set from the manager configuration
	// file's DefaultTimeoutStartSec= option
	TimeoutSec string `json:"timeout_sec"`
}

// Dependencies defines dependencies to other services
type Dependencies struct {
	// Requires configures requirement dependencies on other units.
	// https://www.freedesktop.org/software/systemd/man/systemd.unit.html#Requires=
	Requires string `json:"Requires"`
	// After/Before onfigures ordering dependencies between units.
	// After sets up after order so that if unit A specifies After=B, A's start-up
	// is delayed after B
	// https://www.freedesktop.org/software/systemd/man/systemd.unit.html#Before=
	After string `json:"After"`
	// Before is the inverse of After
	Before string `json:"Before"`
}

type serviceTemplate struct {
	// ServiceSpec specifies the service
	ServiceSpec
	// Name names the mount service
	Name string
	// Descriptions provides an optional service description
	Description string
}

type mountServiceTemplate struct {
	// MountServiceSpec specifies the mount service
	MountServiceSpec
	// Name names the mount service
	Name string
	// Descriptions provides an optional service description
	Description string
}

// PackageServiceStatus provides the status of a running service
type PackageServiceStatus struct {
	// Package identifies the package
	Package loc.Locator
	// Status is a service status
	Status string
}

// ListServiceOptions describes additional configuration for listing
// services.
// An empty value of this type is usable
type ListServiceOptions struct {
	// All optionally indicates whether to query all units (and not only those in memory)
	All bool
	// Type optionally specifies the unit type
	Type string
	// State optionally specifies the unit state
	State string
	// Pattern optionally specifies the unit pattern
	Pattern string
}

const (
	// UnitTypeService defines the service type of the unit file
	UnitTypeService = "service"
)

// ServiceManager is an interface for collaborating with system
// service managers, e.g. systemd for host packages
type ServiceManager interface {
	// IsPackageServiceInstalled checks if the package service is installed
	IsPackageServiceInstalled(pkg loc.Locator) (bool, error)

	// InstallPackageService installs service with system service manager
	InstallPackageService(NewPackageServiceRequest) error

	// UninstallPackageService uninstalls service
	UninstallPackageService(pkg loc.Locator) error

	// DisablePackageService disables service without stopping it
	DisablePackageService(pkg loc.Locator) error

	// EnablePackageService enables service without starting it
	EnablePackageService(pkg loc.Locator) error

	// ListPackageServices lists installed package services
	ListPackageServices(ListServiceOptions) ([]PackageServiceStatus, error)

	// StartPackageService starts package service
	StartPackageService(pkg loc.Locator, noBlock bool) error

	// StopPackageService stops package service
	StopPackageService(pkg loc.Locator) error

	// StopPackageServiceCommand returns command that will stop this service
	StopPackageServiceCommand(pkg loc.Locator) ([]string, error)

	// RestartPackageService restarts package service
	RestartPackageService(pkg loc.Locator) error

	// StatusPackageService returns status of a package service
	StatusPackageService(pkg loc.Locator) (string, error)

	// InstallService installs a service with the system service manager
	InstallService(NewServiceRequest) error

	// InstallMountService installs a new mount service with the system service
	// manager
	InstallMountService(NewMountServiceRequest) error

	// UninstallService uninstalls service
	UninstallService(UninstallServiceRequest) error

	// DisableService disables service without stopping it
	DisableService(DisableServiceRequest) error

	// StartService starts service
	StartService(name string, noBlock bool) error

	// StopService stops service
	StopService(name string) error

	// RestartService restarts service
	RestartService(name string) error

	// StatusService returns status of a service
	StatusService(name string) (string, error)

	// Version returns systemd version
	Version() (int, error)
}

const (
	servicePrefix = "gravity"
)

// New creates a new instance of the default service manager
func New() (ServiceManager, error) {
	_, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &systemdManager{}, nil
}

// CheckAndSetDefaults verifies that this specification object is valid
func (r *MountServiceSpec) CheckAndSetDefaults() error {
	if r.What == "" {
		return trace.BadParameter("What is required")
	}
	if r.Where == "" {
		return trace.BadParameter("Where is required")
	}
	return nil
}

// CheckAndSetDefaults verifies that this request object is valid
func (r *NewMountServiceRequest) CheckAndSetDefaults() error {
	if err := r.ServiceSpec.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.Name == "" {
		return trace.BadParameter("Name is required")
	}
	return nil
}

// CheckAndSetDefaults verifies that this request object is valid
func (r *NewServiceRequest) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("Name is required")
	}
	if r.RestartSec == 0 {
		r.RestartSec = defaults.SystemServiceRestartSec
	}
	return nil
}

// IsUnknownServiceError determines whether the err specifies the
// 'unknown service' error
func IsUnknownServiceError(err error) bool {
	const errCodeNotInstalled = 5
	if exitCode := utils.ExitStatusFromError(err); exitCode != nil {
		return *exitCode == errCodeNotInstalled
	}
	return false
}
