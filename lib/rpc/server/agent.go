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

package server

import (
	"os"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/modules"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// Command executes the command given with req and streams the output of the command as a result
func (srv *AgentServer) Command(req *pb.CommandArgs, stream pb.Agent_CommandServer) error {
	if len(req.Args) == 0 {
		return trace.BadParameter("at least one argument is required")
	}

	log := srv.config.WithFields(log.Fields{
		"request": "Command",
		"args":    req.Args})
	log.Debug("Request received.")

	if req.SelfCommand {
		req.Args = append([]string{utils.Exe.Path}, req.Args...)
		req.WorkingDir = utils.Exe.WorkingDir
	}

	return trace.Wrap(srv.command(*req, stream, log))
}

// PeerJoin accepts a new peer
func (srv *AgentServer) PeerJoin(ctx context.Context, req *pb.PeerJoinRequest) (*types.Empty, error) {
	srv.config.WithField("req", req.String()).Info("PeerJoin.")
	err := srv.config.PeerStore.NewPeer(ctx, *req, &remotePeer{
		addr:             req.Addr,
		creds:            srv.config.Client,
		reconnectTimeout: srv.config.ReconnectTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &types.Empty{}, nil
}

// PeerLeave receives a "leave" request from a peer and initiates its shutdown
func (srv *AgentServer) PeerLeave(ctx context.Context, req *pb.PeerLeaveRequest) (*types.Empty, error) {
	srv.config.WithField("req", req.String()).Info("PeerLeave.")
	err := srv.config.PeerStore.RemovePeer(ctx, *req, &remotePeer{
		addr:             req.Addr,
		creds:            srv.config.Client,
		reconnectTimeout: srv.config.ReconnectTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &types.Empty{}, nil
}

// GetRuntimeConfig returns the agent's runtime configuration
func (srv *AgentServer) GetRuntimeConfig(ctx context.Context, _ *types.Empty) (*pb.RuntimeConfig, error) {
	stateDir := srv.config.StateDir
	if stateDir == "" {
		var err error
		stateDir, err = state.GetStateDir()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	config := &pb.RuntimeConfig{
		Role:          srv.config.RuntimeConfig.Role,
		AdvertiseAddr: srv.config.Listener.Addr().String(),
		SystemDevice:  srv.config.RuntimeConfig.SystemDevice,
		Mounts:        srv.config.RuntimeConfig.Mounts,
		StateDir:      stateDir,
		TempDir:       os.TempDir(),
		KeyValues:     srv.config.RuntimeConfig.KeyValues,
		CloudMetadata: srv.config.RuntimeConfig.CloudMetadata,
		SELinux:       srv.config.RuntimeConfig.SELinux,
	}
	return config, nil
}

// GetSystemInfo queries system information on the host the agent is running on
func (srv *AgentServer) GetSystemInfo(ctx context.Context, _ *types.Empty) (*pb.SystemInfo, error) {
	info, err := srv.config.systemInfo.getSystemInfo()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	payload, err := storage.MarshalSystemInfo(info)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.SystemInfo{Payload: payload}, nil
}

// GetCurrentTime queries the time on the remote node
func (srv *AgentServer) GetCurrentTime(ctx context.Context, _ *types.Empty) (*types.Timestamp, error) {
	ts, err := types.TimestampProto(time.Now().UTC())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ts, nil
}

// GetVersion queries the agent version information
func (srv *AgentServer) GetVersion(ctx context.Context, _ *types.Empty) (*pb.Version, error) {
	ver := modules.Get().Version()
	return &ver, nil
}

// Shutdown requests agent to shut down
func (srv *AgentServer) Shutdown(ctx context.Context, req *pb.ShutdownRequest) (resp *types.Empty, err error) {
	srv.config.WithField("req", req).Info("Shutdown.")
	if srv.config.StopHandler != nil {
		err = srv.config.StopHandler(ctx, req.Completed)
	}
	go func() {
		// Create a separate context from the parent one since the parent
		// context is canceled once the handler has returned
		ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
		if err := srv.Stop(ctx); err != nil {
			srv.config.Warnf("Failed to shutdown: %v.", err)
		}
		cancel()
	}()

	return &types.Empty{}, trace.Wrap(err)
}

// Abort aborts this server. Invokes an abort handler if one has been specified
func (srv *AgentServer) Abort(ctx context.Context, req *types.Empty) (resp *types.Empty, err error) {
	srv.config.Info("Aborting agent.")
	if srv.config.AbortHandler != nil {
		err = srv.config.AbortHandler(ctx)
	}
	go func() {
		// Create a separate context from the parent one since the parent
		// context is canceled once the handler has returned
		ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
		if err := srv.Stop(ctx); err != nil {
			srv.config.Warnf("Failed to stop server: %v.", err)
		}
		cancel()
	}()
	return &types.Empty{}, trace.Wrap(err)
}

func (srv *AgentServer) command(req pb.CommandArgs, stream pb.Agent_CommandServer, log *log.Entry) (err error) {
	err = srv.config.commandExecutor.exec(stream.Context(), stream, req, makeRemoteLogger(stream, srv.config.FieldLogger))
	if err != nil {
		stream.Send(pb.ErrorToMessage(err)) //nolint:errcheck
		log.WithError(err).Warn("Command completed with error.")
		return trace.Wrap(err)
	}
	log.Debug("Command completed OK.")
	return nil
}
