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

package opsservice

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/cloudflare/cfssl/signer"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
)

// signTLSKey signs X509 Public Key with X509 certificate authority of this site
func (s *site) signTLSKey(req ops.TLSSignRequest) (*ops.TLSSignResponse, error) {
	if req.TTL <= 0 || req.TTL > constants.MaxInteractiveSessionTTL {
		req.TTL = constants.MaxInteractiveSessionTTL
	}

	archive, err := s.readCertAuthorityPackage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caKeyPair, err := archive.GetKeyPair(constants.RootKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := authority.ProcessCSR(signer.SignRequest{
		Request: string(req.CSR),
		Subject: req.Subject,
	}, req.TTL, caKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ops.TLSSignResponse{
		Cert:   cert,
		CACert: caKeyPair.CertPEM,
	}, nil
}
