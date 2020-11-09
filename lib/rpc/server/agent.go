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

	"github.com/gravitational/gravity/lib/modules"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// Command executes the command given with req and streams the output of the command as a result
func (srv *agentServer) Command(req *pb.CommandArgs, stream pb.Agent_CommandServer) error {
	if len(req.Args) == 0 {
		return trace.BadParameter("at least one argument is required")
	}

	log := srv.WithFields(log.Fields{
		"request": "Command",
		"args":    req.Args})
	log.Debug("Request received.")

	if req.SelfCommand {
		gravityPath, err := os.Executable()
		if err != nil {
			return trace.ConvertSystemError(err)
		}

		req.Args = append([]string{gravityPath}, req.Args...)
	}

	return trace.Wrap(srv.command(*req, stream, log))
}

// PeerJoin accepts a new peer
func (srv *agentServer) PeerJoin(ctx context.Context, req *pb.PeerJoinRequest) (*types.Empty, error) {
	srv.WithField("req", req).Debug("PeerJoin.")
	err := srv.PeerStore.NewPeer(ctx, *req, &remotePeer{
		addr:             req.Addr,
		creds:            srv.Config.Client,
		reconnectTimeout: srv.Config.ReconnectTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &types.Empty{}, nil
}

// PeerLeave receives a "leave" request from a peer and initiates its shutdown
func (srv *agentServer) PeerLeave(ctx context.Context, req *pb.PeerLeaveRequest) (*types.Empty, error) {
	srv.WithField("req", req).Debug("PeerLeave.")
	err := srv.PeerStore.RemovePeer(ctx, *req, &remotePeer{
		addr:             req.Addr,
		creds:            srv.Config.Client,
		reconnectTimeout: srv.Config.ReconnectTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &types.Empty{}, nil
}

// RuntimeConfig returns the agent's runtime configuration
func (srv *agentServer) GetRuntimeConfig(ctx context.Context, _ *types.Empty) (*pb.RuntimeConfig, error) {
	stateDir := srv.StateDir
	if stateDir == "" {
		var err error
		stateDir, err = state.GetStateDir()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	tempDir := os.TempDir()
	config := &pb.RuntimeConfig{
		Role:          srv.Role,
		AdvertiseAddr: srv.Config.Listener.Addr().String(),
		SystemDevice:  srv.SystemDevice,
		Mounts:        srv.Mounts,
		StateDir:      stateDir,
		TempDir:       tempDir,
		KeyValues:     srv.KeyValues,
		CloudMetadata: srv.CloudMetadata,
	}
	return config, nil
}

// GetSystemInfo queries system information on the host the agent is running on
func (srv *agentServer) GetSystemInfo(ctx context.Context, _ *types.Empty) (*pb.SystemInfo, error) {
	info, err := srv.systemInfo.getSystemInfo()
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
func (srv *agentServer) GetCurrentTime(ctx context.Context, _ *types.Empty) (*types.Timestamp, error) {
	ts, err := types.TimestampProto(time.Now().UTC())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ts, nil
}

// GetVersion queries the agent version information
func (srv *agentServer) GetVersion(ctx context.Context, _ *types.Empty) (*pb.Version, error) {
	ver := modules.Get().Version()
	return &ver, nil
}

// Shutdown requests agent to shut down
func (srv *agentServer) Shutdown(ctx context.Context, req *pb.ShutdownRequest) (resp *types.Empty, err error) {
	srv.WithField("req", req).Info("Shutdown.")
	if srv.StopHandler != nil {
		err = srv.StopHandler(ctx, req.Completed)
	}
	go srv.Stop(ctx)
	return &types.Empty{}, trace.Wrap(err)
}

func (srv *agentServer) Abort(ctx context.Context, req *types.Empty) (resp *types.Empty, err error) {
	srv.Info("Aborting agent.")
	if srv.AbortHandler != nil {
		err = srv.AbortHandler(ctx)
	}
	go srv.Stop(ctx)
	return &types.Empty{}, trace.Wrap(err)
}

func (srv *agentServer) command(req pb.CommandArgs, stream pb.Agent_CommandServer, log *log.Entry) (err error) {
	err = srv.commandExecutor.exec(stream.Context(), stream, req.Args, makeRemoteLogger(stream, srv.FieldLogger))
	if err != nil {
		stream.Send(pb.ErrorToMessage(err))
		log.WithError(err).Warn("Command completed with error.")
	} else {
		log.Debug("Command completed OK.")
	}
	return trace.Wrap(err)
}
