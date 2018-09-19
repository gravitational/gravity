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

package cli

import (
	"time"

	"github.com/gravitational/gravity/lib/localenv"
)

type rollbackParams struct {
	// phaseID is the ID of the phase to rollback
	phaseID string
	// force allows to force phase rollback
	force bool
	// skipVersionCheck allows to override gravity version compatibility check
	skipVersionCheck bool
	// timeout is phase rollback timeout
	timeout time.Duration
}

func rollbackOperationPhase(env, updateEnv, joinEnv *localenv.LocalEnvironment, p rollbackParams) error {
	if hasUpdateOperation(updateEnv) {
		return rollbackUpgradePhase(env, updateEnv, p)
	}
	if joinEnv != nil && hasExpandOperation(joinEnv) {
		return rollbackJoinPhase(env, joinEnv, p)
	}
	return rollbackInstallPhase(env, p)
}
