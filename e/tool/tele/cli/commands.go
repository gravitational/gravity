package cli

import (
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/tool/tele/cli"

	"gopkg.in/alecthomas/kingpin.v2"
)

type Application struct {
	cli.Application
	// BuildCmd is the extended "tele build" command
	BuildCmd BuildCmd
	// LoginCmd logs into specified Ops Center and cluster
	LoginCmd LoginCmd
	// LogoutCmd clears current login information
	LogoutCmd LogoutCmd
	// StatusCmd displays current login information
	StatusCmd StatusCmd
	// PushCmd uploads app installer to Ops Center
	PushCmd PushCmd
	// CreateCmd creates specified resource
	CreateCmd CreateCmd
	// GetCmd shows specified resource
	GetCmd GetCmd
	// RemoveCmd removes specified resource
	RemoveCmd RemoveCmd
}

// BuildCmd builds app installer tarball
type BuildCmd struct {
	*cli.BuildCmd
	// RemoteSupport is the remote support Ops Center to include in tarball
	RemoteSupport *string
	// RemoteSupporToken is the remote support token to include in tarball
	RemoteSupportToken *string
	// CACert is path to certificate authority to include in tarball
	CACert *string
	// EncryptionKey allows to encrypt installer tarball
	EncryptionKey *string
}

// LoginCmd logs into specified Ops Center and cluster
type LoginCmd struct {
	*kingpin.CmdClause
	// Cluster is cluster to log into
	Cluster *string
	// OpsCenter is Ops Center to log into
	OpsCenter *string
	// ConnectorID is connector to use for authentication
	ConnectorID *string
	// TTL is login TTL
	TTL *time.Duration
	// Token is token for non-interactive authentication
	Token *string
}

// LogoutCmd clears current login information
type LogoutCmd struct {
	*kingpin.CmdClause
}

// StatusCmd shows current login information
type StatusCmd struct {
	*kingpin.CmdClause
}

// PushCmd uploads app installer to Ops Center
type PushCmd struct {
	*kingpin.CmdClause
	// Tarball is installer tarball
	Tarball *string
	// Force allows to overwrite existing app
	Force *bool
	// Quiet allows to suppress console output
	Quiet *bool
}

// CreateCmd creates specified resource
type CreateCmd struct {
	*kingpin.CmdClause
	// Filename is the file with resource definition
	Filename *string
	// Force allows to overwrite existing resource
	Force *bool
}

// GetCmd shows specified resource
type GetCmd struct {
	*kingpin.CmdClause
	// Kind is resource kind
	Kind *string
	// Name is resource name
	Name *string
	// Format is output format
	Format *constants.Format
	// Output is output format
	Output *constants.Format
}

// RemoveCmd removes specified resource
type RemoveCmd struct {
	*kingpin.CmdClause
	// Kind is resource kind
	Kind *string
	// Name is resource name
	Name *string
	// Force allows to suppress not found errors
	Force *bool
}
