package environment

import (
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/localenv"

	"github.com/gravitational/trace"
)

// Local extends the LocalEnvironment from open-source
type Local struct {
	*localenv.LocalEnvironment
}

// ClusterOperator returns the enterprise cluster operator client
func (l *Local) ClusterOperator() (*client.Client, error) {
	operator, err := l.LocalEnvironment.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.New(operator), nil
}

// Cluster operator returns the enterprise cluster operator client
func ClusterOperator() (*client.Client, error) {
	operator, err := localenv.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.New(operator), nil
}

// GetCurrentOpsCenter returns the currently active key store entry
func GetCurrentOpsCenter(keyStoreDir string) (string, error) {
	keyStore, err := localenv.GetLocalKeyStore(keyStoreDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	current := keyStore.GetCurrentOpsCenter()
	if current == "" {
		return "", trace.NotFound("not logged in")
	}
	return current, nil
}
