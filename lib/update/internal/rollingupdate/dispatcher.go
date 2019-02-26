/*
Copyright 2019 Gravitational, Inc.

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

// This file implements a rolling update FSM
package rollingupdate

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	libphase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

//  NewDefaultDispatcher returns a new instance of the default phase dispatcher
func NewDefaultDispatcher() Dispatcher {
	return &dispatcher{}
}

// Dispatch returns the appropriate phase executor based on the provided parameters
func (r *dispatcher) Dispatch(config Config, params fsm.ExecutorParams, remote fsm.Remote, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	switch params.Phase.Executor {
	case libphase.UpdateConfig:
		return libphase.NewUpdateConfig(params,
			config.Operator, *config.Operation, config.Apps, config.ClusterPackages,
			config.RequestAdaptor, logger)
	case libphase.RestartContainer:
		return libphase.NewRestart(params, config.Operator, config.Apps, config.Operation.ID,
			logger)
	case libphase.Elections:
		return libphase.NewElections(params, config.Operator, logger)
	case libphase.Drain:
		return libphase.NewDrain(params, config.Client, logger)
	case libphase.Taint:
		return libphase.NewTaint(params, config.Client, logger)
	case libphase.Untaint:
		return libphase.NewUntaint(params, config.Client, logger)
	case libphase.Uncordon:
		return libphase.NewUncordon(params, config.Client, logger)
	case libphase.Endpoints:
		return libphase.NewEndpoints(params, config.Client, logger)
	default:
		return nil, trace.BadParameter("unknown executor %v for phase %q",
			params.Phase.Executor, params.Phase.ID)
	}
}

// RequestAdaptor allows to augment configuration update request
type RequestAdaptor interface {
	// UpdateRequest augments the specified request.
	// Implementations can use this to update (or set additional) fields on the request
	// before dispatching
	UpdateRequest(ops.RotatePlanetConfigRequest, ops.SiteOperation) ops.RotatePlanetConfigRequest
}

// UpdateRequest updates the specified request by invoking itself.
// Implements RequestAdaptor
func (r RequestAdaptorFunc) UpdateRequest(req ops.RotatePlanetConfigRequest, operation ops.SiteOperation) ops.RotatePlanetConfigRequest {
	return r(req, operation)
}

// RequestAdaptorFunc enables a function as a RequestAdaptor
type RequestAdaptorFunc func(ops.RotatePlanetConfigRequest, ops.SiteOperation) ops.RotatePlanetConfigRequest

// Dispatcher routes the set of execution parameters to a specific operation phase
type Dispatcher interface {
	// Dispatch returns an executor for the given parameters and the specified remote
	Dispatch(Config, fsm.ExecutorParams, fsm.Remote, log.FieldLogger) (fsm.PhaseExecutor, error)
}

// idRequest passes the specified request unmodified
func idRequest(req ops.RotatePlanetConfigRequest, operation ops.SiteOperation) ops.RotatePlanetConfigRequest {
	return req
}

// Dispatch returns the appropriate phase executor based on the provided parameters
func (r updateDispatcher) Dispatch(params fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: log.WithFields(log.Fields{
			constants.FieldPhase: params.Phase.ID,
		}),
		Key:      params.Key(),
		Operator: r.Operator,
	}
	if params.Phase.Data != nil {
		logger.Server = params.Phase.Data.Server
	}
	return r.Dispatcher.Dispatch(r.Config, params, remote, logger)
}

// updateDispatcher is a convenience implementation that dispatches to the underlying
// instance.
// Implements update.Dispatcher
type updateDispatcher struct {
	Config
	Dispatcher
}

// dispatcher routes parameters to a specific operation phase.
// Implements Dispatcher
type dispatcher struct{}
