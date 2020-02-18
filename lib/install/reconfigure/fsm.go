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

package reconfigure

import (
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/engine"
	"github.com/gravitational/gravity/lib/ops"
)

// NewFSMFactory returns a new factory that can create fsm for the reconfigure operation.
func NewFSMFactory(config install.Config) engine.FSMFactory {
	return &fsmFactory{Config: config}
}

// NewFSM creates a new fsm for the provided operator and operation.
func (f *fsmFactory) NewFSM(operator ops.Operator, operationKey ops.SiteOperationKey) (*fsm.FSM, error) {
	fsmConfig := install.NewFSMConfig(operator, operationKey, f.Config)
	fsmConfig.Spec = FSMSpec(fsmConfig)
	return install.NewFSM(fsmConfig)
}

type fsmFactory struct {
	install.Config
}
