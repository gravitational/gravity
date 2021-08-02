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

package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/encryptedpack"

	"github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewDecrypt returns a new "decrypt" phase executor
func NewDecrypt(p fsm.ExecutorParams, operator ops.Operator, packages pack.PackageService, apps app.Applications) (fsm.PhaseExecutor, error) {
	application, err := apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
	}
	return &decryptExecutor{
		FieldLogger:    logger,
		Packages:       packages,
		Apps:           apps,
		Application:    *application,
		ExecutorParams: p,
	}, nil
}

type decryptExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Packages is the installer process pack service
	Packages pack.PackageService
	// Apps is the install process app service
	Apps app.Applications
	// Application is the application being installed
	Application app.Application
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the decrypt phase
func (p *decryptExecutor) Execute(ctx context.Context) error {
	deps, err := app.GetDependencies(app.GetDependenciesRequest{
		App:  p.Application,
		Apps: p.Apps,
		Pack: p.Packages,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// decrypt all application dependencies, the application package
	// itself and the certificate authority package
	locators := append(deps.AsPackages(),
		p.Application.Package, loc.OpsCenterCertificateAuthority)
	for _, locator := range locators {
		p.Progress.NextStep("Decrypting package %v:%v", locator.Name, locator.Version)
		p.Infof("Decrypting package %v:%v.", locator.Name, locator.Version)
		err := encryptedpack.DecryptPackage(p.Packages, locator, p.Phase.Data.Data)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Rollback is no-op for this phase
func (*decryptExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*decryptExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*decryptExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// GetEncryptionKey extracts encryption key from the provided license string
func GetEncryptionKey(lic string) (string, error) {
	if lic == "" {
		return "", trace.NotFound("no license found")
	}
	parsed, err := license.ParseLicense(lic)
	if err != nil {
		return "", trace.Wrap(err)
	}
	key := string(parsed.GetPayload().EncryptionKey)
	if key == "" {
		return "", trace.NotFound("license does not contain encryption key")
	}
	return key, nil
}
