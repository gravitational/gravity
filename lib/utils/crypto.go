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

package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
)

// GeneratePrivateKeyPEM generates and returns PEM serialzed
func GeneratePrivateKeyPEM() ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, defaults.RSAPrivateKeyBits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyPEM := pem.EncodeToMemory(
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if keyPEM == nil {
		return nil, trace.BadParameter("failed to encode key")
	}
	return keyPEM, nil
}
