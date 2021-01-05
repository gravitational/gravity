package install

import (
	"fmt"

	"github.com/gravitational/gravity/e/lib/install/phases"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	ossphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// PlanBuilder extends the open-source plan builder
type PlanBuilder struct {
	*install.PlanBuilder
}

// AddInstallerPhase appends installer download phase to the provided plan
func (b *PlanBuilder) AddInstallerPhase(plan *storage.OperationPlan, opsCluster, opsURL, opsToken string) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID: phases.InstallerPhase,
		Description: fmt.Sprintf("Download installer from Ops Center %v",
			opsCluster),
		Data: &storage.OperationPhaseData{
			Package: &b.Application.Package,
			Agent: &storage.LoginEntry{
				OpsCenterURL: opsURL,
				Password:     opsToken,
			},
		},
		Requires: fsm.RequireIfPresent(plan, ossphases.ChecksPhase),
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
		Requires: fsm.RequireIfPresent(plan, phases.InstallerPhase),
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
		Description: fmt.Sprintf("Connect to Ops Center %v",
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
