package status

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"

	humanize "github.com/dustin/go-humanize"
	"github.com/gravitational/trace"
)

// Status is the extension to the open-source cluster status
type Status struct {
	// Updates describes the configured periodic updates
	Updates *ops.PeriodicUpdatesStatusResponse `json:"updates,omitempty"`
	// AccessStatus describes external Ops Centers tunnel status
	AccessStatus []OpsCenterStatus `json:"access_links,omitempty"`
	// TrustedClusterToken is used by remote clusters connecting to Ops Center
	TrustedClusterToken storage.Token `json:"trustedClusterToken,omitempty"`
}

// Collect gathers extended enterprise-specific cluster status information
// such as remote support and periodic updates information
func (s *Status) Collect(ctx context.Context) error {
	clusterEnv, err := localenv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	operator := service.New(clusterEnv.Operator)
	cluster, err := operator.GetLocalSite(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	s.TrustedClusterToken, err = operator.GetTrustedClusterToken(cluster.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	trustedCluster, err := ops.GetTrustedCluster(cluster.Key(), operator)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trustedCluster != nil {
		s.AccessStatus = append(s.AccessStatus, OpsCenterStatus{
			Hostname: trustedCluster.GetName(),
			Enabled:  trustedCluster.GetEnabled(),
		})
	}
	s.Updates, err = operator.PeriodicUpdatesStatus(cluster.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// WriteTo writes the collected extended enterprise status into provided writer
func (s *Status) WriteTo(w io.Writer) (n int64, err error) {
	var errors []error
	if s.TrustedClusterToken != nil {
		errors = append(errors, s.fprintf(&n, w, "Trusted cluster token:\t%v\n",
			s.TrustedClusterToken.GetName()))
	}
	if s.Updates == nil {
		errors = append(errors, s.fprintf(&n, w, "Periodic updates:\tNot Configured\n"))
	} else if s.Updates.Enabled {
		errors = append(errors, s.fprintf(&n, w, "Periodic updates:\tON, every %v, next check at %v (%v)\n",
			s.Updates.Interval, s.Updates.NextCheck.Format(constants.HumanDateFormat),
			humanize.RelTime(time.Now(), s.Updates.NextCheck, "from now", "")))
	} else {
		errors = append(errors, s.fprintf(&n, w, "Periodic updates:\tOFF\n"))
	}
	if len(s.AccessStatus) == 0 {
		errors = append(errors, s.fprintf(&n, w, "Remote support:\tNot Configured\n"))
	} else {
		errors = append(errors, s.fprintf(&n, w, "Remote support:\n"))
		for _, tunnel := range s.AccessStatus {
			if tunnel.Enabled {
				errors = append(errors, s.fprintf(&n, w, "    %v: ON\n", tunnel.Hostname))
			} else {
				errors = append(errors, s.fprintf(&n, w, "    %v: OFF\n", tunnel.Hostname))
			}
		}
	}
	return n, trace.NewAggregate(errors...)
}

func (s *Status) fprintf(n *int64, w io.Writer, format string, a ...interface{}) error {
	written, err := fmt.Fprintf(w, format, a...)
	*n += int64(written)
	return trace.Wrap(err)
}

// init initializes the enterprise cluster status extension
func init() {
	status.SetExtensionFunc(func() status.Extension {
		return &Status{}
	})
}

// OpsCenterStatus specifies the status of an external Ops Center connection
type OpsCenterStatus struct {
	// Hostname of the Ops Center
	Hostname string `json:"hostname"`
	// Enabled specifies whether the tunnel is on
	Enabled bool `json:"enabled"`
}
