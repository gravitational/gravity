/*
Copyright 2020 Gravitational, Inc.

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

package storage

import (
	"github.com/gravitational/gravity/lib/compare"

	"gopkg.in/check.v1"
)

type PlanSuite struct{}

var _ = check.Suite(&PlanSuite{})

func (*PlanSuite) TestGetLeafPhases(c *check.C) {
	initPhase := OperationPhase{ID: "/init"}
	bootstrapPhase1 := OperationPhase{ID: "/bootstrap/node-1"}
	bootstrapPhase2 := OperationPhase{ID: "/bootstrap/node-2"}
	bootstrapPhase := OperationPhase{
		ID:     "/bootstrap",
		Phases: []OperationPhase{bootstrapPhase1, bootstrapPhase2},
	}
	upgradePhase := OperationPhase{ID: "/upgrade"}
	plan := &OperationPlan{
		Phases: []OperationPhase{initPhase, bootstrapPhase, upgradePhase},
	}
	compare.DeepCompare(c, plan.GetLeafPhases(), []OperationPhase{
		initPhase, bootstrapPhase1, bootstrapPhase2, upgradePhase})
}
