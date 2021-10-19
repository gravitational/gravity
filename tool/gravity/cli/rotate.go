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
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
)

type rotateOptions struct {
	// clusterName is the local cluster name
	clusterName string
	// validFor specifies validity duration for renewed certs
	validFor time.Duration
	// caPath is optional CA to use
	caPath string
}

func rotateCertificates(env *localenv.LocalEnvironment, o rotateOptions) (err error) {
	var archive utils.TLSArchive
	if o.caPath != "" {
		archive, err = readCertAuthorityFromFile(o.caPath)
		env.Printf("Using certificate authority from %v\n", o.caPath)
	} else {
		archive, err = readCertAuthorityPackage(env.Packages, o.clusterName)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	caKeyPair, err := archive.GetKeyPair(constants.RootKeyPair)
	if err != nil {
		return trace.Wrap(err)
	}
	baseKeyPair, err := archive.GetKeyPair(constants.APIServerKeyPair)
	if err != nil {
		return trace.Wrap(err)
	}
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = backupSecrets(env, state.SecretDir(stateDir))
	if err != nil {
		return trace.Wrap(err)
	}
	for _, certName := range certNames {
		// read x509 cert from disk
		cert, err := readCertificate(state.Secret(stateDir, certName+".cert"))
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			env.Printf("Certificate %v.cert is not present\n", certName)
			continue
		}
		env.Printf("Renewing certificate %v.cert\n", certName)
		// copy all data from the old cert into the new csr
		req := certToCSR(cert)
		// generate a new key pair
		keyPair, err := authority.GenerateCertificate(req, caKeyPair, baseKeyPair.KeyPEM, o.validFor)
		if err != nil {
			return trace.Wrap(err)
		}
		// save new key pair
		err = ioutil.WriteFile(state.Secret(stateDir, certName+".cert"), keyPair.CertPEM, defaults.SharedReadMask)
		if err != nil {
			return trace.Wrap(err)
		}
		err = ioutil.WriteFile(state.Secret(stateDir, certName+".key"), keyPair.KeyPEM, defaults.SharedReadMask)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func exportCertificateAuthority(env *localenv.LocalEnvironment, clusterName, path string) error {
	archive, err := readCertAuthorityPackage(env.Packages, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	reader, err := utils.CreateTLSArchive(archive)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ioutil.WriteFile(path, bytes, defaults.SharedReadMask)
	if err != nil {
		return trace.Wrap(err)
	}
	env.Printf("Certificate authority exported to %v\n", path)
	return nil
}

// backupSecrets backs up the contents of the specified dir
func backupSecrets(env *localenv.LocalEnvironment, path string) error {
	suffix, err := users.CryptoRandomToken(6)
	if err != nil {
		return trace.Wrap(err)
	}
	backupPath := fmt.Sprintf("%v-%v", path, suffix)
	err = utils.CopyDirContents(path, backupPath)
	if err != nil {
		return trace.Wrap(err)
	}
	env.Printf("Backed up %v to %v\n", path, backupPath)
	return nil
}

// readCertificate returns the parsed x509 cert from the provided path
func readCertificate(path string) (*x509.Certificate, error) {
	certBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return nil, trace.BadParameter("failed to decode certificate at %v", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cert, nil
}

// certToCSR creates a new CSR using the data from the provided certificate
func certToCSR(cert *x509.Certificate) csr.CertificateRequest {
	req := csr.CertificateRequest{
		CN:    cert.Subject.CommonName,
		Hosts: cert.DNSNames,
	}
	for _, ip := range cert.IPAddresses {
		req.Hosts = append(req.Hosts, ip.String())
	}
	for _, o := range cert.Subject.Organization {
		req.Names = append(req.Names, csr.Name{O: o})
	}
	return req
}

func readCertAuthorityPackage(packages pack.PackageService, clusterName string) (utils.TLSArchive, error) {
	locator, err := loc.ParseLocator(fmt.Sprintf("%v/%v:0.0.1", clusterName,
		constants.CertAuthorityPackage))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, reader, err := packages.ReadPackage(*locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	return utils.ReadTLSArchive(reader)
}

func readCertAuthorityFromFile(path string) (utils.TLSArchive, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return utils.ReadTLSArchive(bytes.NewBuffer(data))
}

var certNames = []string{
	constants.APIServerKeyPair,
	constants.ETCDKeyPair,
	constants.KubeletKeyPair,
	constants.ProxyKeyPair,
	constants.SchedulerKeyPair,
	constants.ControllerManagerKeyPair,
	constants.KubectlKeyPair,
}
