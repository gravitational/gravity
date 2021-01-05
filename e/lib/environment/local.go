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

// ClusterOperator returns the enterprise cluster operator client
func ClusterOperator() (*client.Client, error) {
	operator, err := localenv.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.New(operator), nil
}
