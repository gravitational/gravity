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

package service

import (
	"sync"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/defaults"
	ossops "github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
)

// installGroup holds information about which agent is the installer for
// a certain install operation
//
// It is used in Ops Center initiated installation when agents on remote
// nodes start simultaneously and need to decide which one of them will
// be the installer and which ones will be joining.
//
// The installer IP is stored with a TTL, this way agents can detect that
// the installer process for example has shutdown.
type installGroup struct {
	// Mutex is used for ensure atomicity when registering agents
	sync.Mutex
	// key is the operation key the install group is for
	key ossops.SiteOperationKey
	// m is the TTL map that holds registration request of the installer node
	m *ttlmap.TTLMap
}

// newInstallGroup initializes a new install group for the specified operation
func newInstallGroup(key ossops.SiteOperationKey) (*installGroup, error) {
	// the map stores a single element - the installer's registration request
	m, err := ttlmap.New(1)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &installGroup{key: key, m: m}, nil
}

// registerAgent handles registration from the remote node with the provided IP:
//   - if this is the first node in the group, sets it as installer IP and
//     returns the same IP to let the node know that it should act as installer
//   - if there is already an installer IP in the map, returns it to let
//     the node know that it should be joining to this installer
func (g *installGroup) registerAgent(req ops.RegisterAgentRequest) (*ops.RegisterAgentResponse, error) {
	g.Lock()
	defer g.Unlock()
	cachedReqI, ok := g.m.Get("installer")
	if !ok {
		// install group is empty meaning this is the first agent to reach
		// out to the Ops Center and it will become an installer
		err := g.m.Set("installer", req, defaults.InstallGroupTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &ops.RegisterAgentResponse{
			InstallerID: req.AgentID,
			InstallerIP: req.AdvertiseIP,
		}, nil
	}
	cachedReq, ok := cachedReqI.(ops.RegisterAgentRequest)
	if !ok {
		return nil, trace.BadParameter(
			"expected RegisterAgentRequest, got: %T", cachedReq)
	}
	if cachedReq.AgentID == req.AgentID {
		// this is the installer re-registering so update TTL
		err := g.m.Set("installer", req, defaults.InstallGroupTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &ops.RegisterAgentResponse{
		InstallerID: cachedReq.AgentID,
		InstallerIP: cachedReq.AdvertiseIP,
	}, nil
}
