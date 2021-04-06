// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/tool/gravity/cli"

	"gopkg.in/alecthomas/kingpin.v2"
)

// Application is the extended gravity CLI application
type Application struct {
	*cli.Application
	// InstallCmd is the extended "gravity install" command
	InstallCmd InstallCmd
	// StatusCmd is the extended "gravity status" command
	StatusCmd StatusCmd
	// UpdateDownloadCmd manages new app versions downloads
	UpdateDownloadCmd UpdateDownloadCmd
	// OpsGenerateCmd generates a standalone installer via an Ops Center
	OpsGenerateCmd OpsGenerateCmd
	// TunnelCmd combines support tunnel related subcommands
	TunnelCmd TunnelCmd
	// TunnelEnableCmd turns remote support on
	TunnelEnableCmd TunnelEnableCmd
	// TunnelDisableCmd turns remote support off
	TunnelDisableCmd TunnelDisableCmd
	// TunnelStatusCmd shows remote support status
	TunnelStatusCmd TunnelStatusCmd
	// LicenseCmd combines subcommands for licenses
	LicenseCmd LicenseCmd
	// LicenseInstallCmd installs license into cluster
	LicenseInstallCmd LicenseInstallCmd
	// LicenseNewCmd generates new license
	LicenseNewCmd LicenseNewCmd
	// LicenseShowCmd displays currently installed license
	LicenseShowCmd LicenseShowCmd
}

// InstallCmd is the extended "gravity install" command
type InstallCmd struct {
	*cli.InstallCmd
	// License is app license
	License *string
	// LicenseFile is path to the license file.
	LicenseFile *string
	// OpsAdvertiseAddr is advertise address for Ops Center
	//
	// TODO(r0mant): REMOVE IN 7.0. This flag is obsolete.
	OpsAdvertiseAddr *string
	// HubAdvertiseAddr is advertise address of Gravity Hub
	HubAdvertiseAddr *string
	// OperationID is existing operation ID
	OperationID *string
	// OpsCenterURL is remote Ops Center URL
	OpsCenterURL *string
	// OpsCenterToken is remote Ops Center token
	OpsCenterToken *string
	// OpsCenterTunnelToken is remote Ops Center gatekeeper token
	OpsCenterTunnelToken *string
	// OpsCenterSNIHost is remote Ops Center SNI host
	OpsCenterSNIHost *string
}

// StatusCmd is the extended "gravity status" command
type StatusCmd struct {
	*cli.StatusCmd
	// Tunnel displays only remote support status
	Tunnel *bool
}

// UpdateDownloadCmd manages new app version downloads
type UpdateDownloadCmd struct {
	*kingpin.CmdClause
	// Every adjusts update check interval
	Every *string
}

// OpsGenerateCmd generates an installer tarball
type OpsGenerateCmd struct {
	*kingpin.CmdClause
	// Package is app locator
	Package *loc.Locator
	// Dir is the directory where installer files will be written to
	Dir *string
	// CACert is certificate authority to put into installer
	CACert *string
	// EncryptionKey encrypts installer packages
	EncryptionKey *string
	// OpsCenterURL is the operator service URL
	OpsCenterURL *string
}

// TunnelCmd combines support tunnel related subcommands
type TunnelCmd struct {
	*kingpin.CmdClause
}

// TunnelEnableCmd turns remote support on
type TunnelEnableCmd struct {
	*kingpin.CmdClause
}

// TunnelDisableCmd turns remote support off
type TunnelDisableCmd struct {
	*kingpin.CmdClause
}

// TunnelStatusCmd shows remote support status
type TunnelStatusCmd struct {
	*kingpin.CmdClause
}

// LicenseCmd combines subcommands for licenses
type LicenseCmd struct {
	*kingpin.CmdClause
}

// LicenseInstallCmd installs license into cluster
type LicenseInstallCmd struct {
	*kingpin.CmdClause
	// Path is license file path
	Path *string
}

// LicenseNewCmd generates new license
type LicenseNewCmd struct {
	*kingpin.CmdClause
	// MaxNodes is permitted cluster size
	MaxNodes *int
	// ValidFor is license validity period
	ValidFor *string
	// StopApp stops app when license expires
	StopApp *bool
	// CACert is certificate authority certificate
	CACert *string
	// CAKey is certificate authority private key
	CAKey *string
	// EncryptionKey is encryption key to encode
	EncryptionKey *string
	// CustomerName is name of customer the license is for
	CustomerName *string
	// CustomerEmail is email of customer the license is for
	CustomerEmail *string
	// CustomerMetadata is additional metadata
	CustomerMetadata *string
	// ProductName is product the license is for
	ProductName *string
	// ProductVersion is product the license is for
	ProductVersion *string
}

// LicenseShowCmd displays installed license
type LicenseShowCmd struct {
	*kingpin.CmdClause
	// Output is output format
	Output *constants.Format
}
