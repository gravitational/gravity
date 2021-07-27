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

package install

import (
	"fmt"

	"github.com/gravitational/gravity/e/lib/install/phases"
	"github.com/gravitational/gravity/lib/fsm"
	ossinstall "github.com/gravitational/gravity/lib/install"
	ossphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// PlanBuilder extends the open-source plan builder
type PlanBuilder struct {
	*ossinstall.PlanBuilder
}

// AddInstallerPhase appends installer download phase to the provided plan
func (b *PlanBuilder) AddInstallerPhase(plan *storage.OperationPlan, opsCluster, opsURL, opsToken string) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID: phases.InstallerPhase,
		Description: fmt.Sprintf("Download installer from Gravity Hub %v",
			opsCluster),
		Data: &storage.OperationPhaseData{
			Package: &b.Application.Package,
			Agent: &storage.LoginEntry{
				OpsCenterURL: opsURL,
				Password:     opsToken,
			},
		},
		Requires: fsm.RequireIfPresent(*plan, ossphases.ChecksPhase),
		Step:     0,
	})
}

// AddDecryptPhase appends package decryption phase to the provided plan
func (b *PlanBuilder) AddDecryptPhase(plan *storage.OperationPlan, encryptionKey string) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.DecryptPhase,
		Description: "Decrypt installer packages",
		Data: &storage.OperationPhaseData{
			Package: &b.Application.Package,
			Data:    encryptionKey,
		},
		Requires: fsm.RequireIfPresent(*plan, ossphases.ChecksPhase),
		Step:     3,
	})
}

// AddLicensePhase appends license installation phase to the provided plan
func (b *PlanBuilder) AddLicensePhase(plan *storage.OperationPlan, license string) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.LicensePhase,
		Description: "Install the cluster license",
		Data: &storage.OperationPhaseData{
			Server:  &b.Master,
			License: []byte(license),
		},
		Requires: []string{ossphases.RBACPhase},
		Step:     4,
	})
}

// AddConnectPhase appends Ops Center connect phase to the provided plan
func (b *PlanBuilder) AddConnectPhase(plan *storage.OperationPlan, trustedCluster storage.TrustedCluster) error {
	bytes, err := storage.MarshalTrustedCluster(trustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID: phases.ConnectPhase,
		Description: fmt.Sprintf("Connect to Gravity Hub %v",
			trustedCluster.GetName()),
		Data: &storage.OperationPhaseData{
			Server:         &b.Master,
			TrustedCluster: bytes,
		},
		Requires: []string{ossphases.RuntimePhase},
		Step:     8,
	})
	return nil
}
