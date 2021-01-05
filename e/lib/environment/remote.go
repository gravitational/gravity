package environment

import (
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/localenv"

	"github.com/gravitational/trace"
)

// Remote extends the RemoteEnvironment from open-source
type Remote struct {
	// RemoteEnvironment is the wrapped open-source remote env
	*localenv.RemoteEnvironment
	// Operator is the enterprise ops web client
	Operator *client.Client
}

// LoginRemote creates new remote environment
func LoginRemote(url, token string) (*Remote, error) {
	ossRemote, err := localenv.LoginRemote(url, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Remote{
		RemoteEnvironment: ossRemote,
		Operator:          client.New(ossRemote.Operator),
	}, nil
}
