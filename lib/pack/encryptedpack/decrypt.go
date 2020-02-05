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

package encryptedpack

import (
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// DecryptPackage decrypts the specified package in the specified package service
func DecryptPackage(p pack.PackageService, loc loc.Locator, encryptionKey string) error {
	envelope, _, err := p.ReadPackage(loc)
	if err != nil {
		return trace.Wrap(err)
	}

	if !envelope.Encrypted {
		log.Infof("%v is not encrypted, nothing to do", loc)
		return nil
	}

	log.Infof("decrypting %v", loc)

	// wrap the package service into "encrypted pack" to decrypt the package
	encryptedPack := New(p, encryptionKey)
	envelope, data, err := encryptedPack.ReadPackage(loc)
	if err != nil {
		return trace.Wrap(err)
	}

	options := envelope.Options()
	options = append(options, pack.WithEncrypted(false))

	// update the decrypted package in the original package service
	_, err = p.UpsertPackage(loc, data, options...)
	return trace.Wrap(err)
}
