package rollingupdate

import (
	"fmt"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	libphase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"
)

// Config creates a new phase to update runtime container configuration
func (r Builder) Config(rootText string) *update.Phase {
	phase := update.RootPhase(update.Phase{
		ID:          "update-config",
		Executor:    libphase.UpdateConfig,
		Description: rootText,
		Data: &storage.OperationPhaseData{
			Package: &r.App,
		},
	})
	return &phase
}

// Masters returns a new phase to rolling update the specified list of master servers
func (r Builder) Masters(servers []storage.Server, rootText, nodeTextFormat string) *update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "masters",
		Description: rootText,
	})
	first, others := servers[0], servers[1:]

	node := r.node(first, first.Hostname, nodeTextFormat)
	if len(others) != 0 {
		node.AddSequential(setLeaderElection(enable(), disable(first), first,
			"stepdown", "Step down %q as Kubernetes leader"))
	}
	node.AddSequential(r.common(&first, nil)...)
	if len(others) != 0 {
		node.AddSequential(setLeaderElection(enable(first), disable(others...), first,
			"elect", "Make node %q Kubernetes leader"))
	}
	root.AddSequential(node)
	for i, server := range others {
		node := r.node(server, server.Hostname, nodeTextFormat)
		node.AddSequential(r.common(&others[i], nil)...)
		node.AddSequential(setLeaderElection(enable(server), disable(), server,
			"enable-elections", "Enable leader election on node %q"))
		root.AddSequential(node)
	}
	return &root
}

// Nodes returns a new phase to rolling update the specified list of regular servers
func (r Builder) Nodes(servers []storage.Server, master *storage.Server, rootText, nodeTextFormat string) *update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "nodes",
		Description: rootText,
	})
	for i, server := range servers {
		node := r.node(server, server.Hostname, nodeTextFormat)
		node.AddSequential(r.common(&servers[i], master)...)
		root.AddSequential(node)
	}
	return &root
}

func (r Builder) common(server, master *storage.Server) (phases []update.Phase) {
	phases = append(phases,
		r.drain(server, master),
		r.restart(server),
		r.taint(server, master),
		r.uncordon(server, master),
		r.endpoints(server, master),
		r.untaint(server, master),
	)
	return phases
}

func (r Builder) restart(server *storage.Server) update.Phase {
	node := r.node(*server, "restart", "Restart container on node %q")
	node.Executor = libphase.RestartContainer
	node.Data = &storage.OperationPhaseData{
		Server:  server,
		Package: &r.App,
	}
	return node
}

func (r Builder) taint(server, execer *storage.Server) update.Phase {
	node := r.node(*server, "taint", "Taint node %q")
	node.Executor = libphase.Taint
	node.Data = &storage.OperationPhaseData{
		Server: server,
	}
	if execer != nil {
		node.Data.ExecServer = execer
	}
	return node
}

func (r Builder) untaint(server, execer *storage.Server) update.Phase {
	node := r.node(*server, "untaint", "Remove taint from node %q")
	node.Executor = libphase.Untaint
	node.Data = &storage.OperationPhaseData{
		Server: server,
	}
	if execer != nil {
		node.Data.ExecServer = execer
	}
	return node
}

func (r Builder) uncordon(server, execer *storage.Server) update.Phase {
	node := r.node(*server, "uncordon", "Uncordon node %q")
	node.Executor = libphase.Uncordon
	node.Data = &storage.OperationPhaseData{
		Server: server,
	}
	if execer != nil {
		node.Data.ExecServer = execer
	}
	return node
}

func (r Builder) endpoints(server, execer *storage.Server) update.Phase {
	node := r.node(*server, "endpoints", "Wait for endpoints on node %q")
	node.Executor = libphase.Endpoints
	node.Data = &storage.OperationPhaseData{
		Server: server,
	}
	if execer != nil {
		node.Data.ExecServer = execer
	}
	return node
}

func (r Builder) drain(server, execer *storage.Server) update.Phase {
	node := r.node(*server, "drain", "Drain node %q")
	node.Executor = libphase.Drain
	node.Data = &storage.OperationPhaseData{
		Server: server,
	}
	if execer != nil {
		node.Data.ExecServer = execer
	}
	return node
}

func (r Builder) node(server storage.Server, id, format string) update.Phase {
	return update.Phase{
		ID:          id,
		Description: fmt.Sprintf(format, server.Hostname),
	}
}

// Builder builds an operation plan
type Builder struct {
	// App specifies the cluster application
	App loc.Locator
}

// setLeaderElection creates a phase that will change the leader election state in the cluster
// enable - the list of servers to enable election on
// disable - the list of servers to disable election on
// server - The server the phase should be executed on, and used to name the phase
// key - is the identifier of the phase (combined with server.Hostname)
// msg - is a format string used to describe the phase
func setLeaderElection(enable, disable []storage.Server, server storage.Server, id, format string) update.Phase {
	return update.Phase{
		ID:          id,
		Executor:    libphase.Elections,
		Description: fmt.Sprintf(format, server.Hostname),
		Data: &storage.OperationPhaseData{
			Server: &server,
			ElectionChange: &storage.ElectionChange{
				EnableServers:  enable,
				DisableServers: disable,
			},
		},
	}
}

func enable(servers ...storage.Server) []storage.Server  { return servers }
func disable(servers ...storage.Server) []storage.Server { return servers }
