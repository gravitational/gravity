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

package rpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strconv"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
	"google.golang.org/grpc/credentials"
)

// GenerateAgentCredentialsPackage creates or updates a package in packages with client/server credentials.
// pkgTemplate specifies the naming template for the resulting package
func GenerateAgentCredentialsPackage(packages pack.PackageService, pkgTemplate loc.Locator,
	archive utils.TLSArchive) (secretsLocator *loc.Locator, err error) {
	secretsLocator, err = loc.NewLocator(
		pkgTemplate.Repository,
		defaults.RPCAgentSecretsPackage,
		pkgTemplate.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = upsertPackage(packages, *secretsLocator, archive)
	if err != nil {
		return secretsLocator, trace.Wrap(err)
	}

	return secretsLocator, nil
}

// LoadCredentials returns the RPC credentials package as a TLS archive
func LoadCredentials(packages pack.PackageService) (tls utils.TLSArchive, err error) {
	_, reader, err := packages.ReadPackage(loc.RPCSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	tlsArchive, err := utils.ReadTLSArchive(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsArchive, nil
}

// GenerateAgentCredentials creates client/server credentials archive.
// hosts lists additional hosts to add to the generated certificates.
func GenerateAgentCredentials(hosts []string, commonName string, longLivedClient bool) (archive utils.TLSArchive, err error) {
	ca, err := authority.GenerateSelfSignedCA(csr.CertificateRequest{
		CN: commonName,
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverKeyPair, err := authority.GenerateCertificate(csr.CertificateRequest{
		CN: pb.ServerName,
		// Go 1.9 https://github.com/golang/go/commit/630e93ed2d8a13226903451a0e85e62efd78cdcd
		Hosts: append(hosts, pb.ServerName),
		Names: []csr.Name{
			{
				O:  "Gravitational",
				OU: "Local Cluster",
			},
		},
	}, ca, nil, defaults.CertificateExpiry)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var clientTTL time.Duration
	if longLivedClient {
		clientTTL = defaults.CertificateExpiry
	}
	clientKeyPair, err := authority.GenerateCertificate(csr.CertificateRequest{
		CN: "leadagent",
		Names: []csr.Name{
			{
				O:  "Gravitational",
				OU: "Local Cluster",
			},
		},
	}, ca, nil, clientTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCert := *ca
	caCert.KeyPEM = nil

	archive = utils.TLSArchive{
		pb.Server: serverKeyPair,
		pb.Client: clientKeyPair,
		pb.CA:     &caCert,
	}

	return archive, nil
}

// ClientCredentialsFromKeyPairs loads agent client credentials from the specified
// set of key pairs
func ClientCredentialsFromKeyPairs(keys, caKeys authority.TLSKeyPair) (credentials.TransportCredentials, error) {
	cert, err := tls.X509KeyPair(keys.CertPEM, keys.KeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caKeys.CertPEM); !ok {
		return nil, trace.BadParameter("failed to add CA to pool")
	}

	creds := credentials.NewTLS(&tls.Config{
		ServerName:   pb.ServerName,
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
	})
	return creds, nil
}

// ValidateCredentials checks the credentials from the specified archive for validity
func ValidateCredentials(archive utils.TLSArchive, now time.Time) error {
	clientKeyPair := archive[pb.Client]
	if err := validateCertificateExpiration(clientKeyPair.CertPEM, now); err != nil {
		return trace.Wrap(err, "invalid client certificate")
	}
	serverKeyPair := archive[pb.Server]
	if err := validateCertificateExpiration(serverKeyPair.CertPEM, now); err != nil {
		return trace.Wrap(err, "invalid server certificate")
	}
	caKeyPair := archive[pb.CA]
	if err := validateCertificateExpiration(caKeyPair.CertPEM, now); err != nil {
		return trace.Wrap(err, "invalid CA certificate")
	}
	return nil
}

// Credentials returns both server and client credentials read from the
// specified directory
func Credentials(packages pack.PackageService) (server credentials.TransportCredentials, client credentials.TransportCredentials, err error) {
	_, reader, err := packages.ReadPackage(loc.RPCSecrets)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer reader.Close()
	tlsArchive, err := utils.ReadTLSArchive(reader)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	server, err = ServerCredentialsFromKeyPairs(*tlsArchive[pb.Server],
		*tlsArchive[pb.CA])
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	client, err = ClientCredentialsFromKeyPairs(*tlsArchive[pb.Client],
		*tlsArchive[pb.CA])
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return server, client, nil
}

// ClientCredentials reads client credentials from specified packages
func ClientCredentials(packages pack.PackageService) (credentials.TransportCredentials, error) {
	_, reader, err := packages.ReadPackage(loc.RPCSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	tlsArchive, err := utils.ReadTLSArchive(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ClientCredentialsFromKeyPairs(*tlsArchive[pb.Client],
		*tlsArchive[pb.CA])
}

// ServerCredentials reads server credentials from the specified package
func ServerCredentials(packages pack.PackageService) (credentials.TransportCredentials, error) {
	_, reader, err := packages.ReadPackage(loc.RPCSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	tlsArchive, err := utils.ReadTLSArchive(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ServerCredentialsFromKeyPairs(*tlsArchive[pb.Server],
		*tlsArchive[pb.CA])
}

// ServerCredentialsFromKeyPairs loads server agent credentials from the specified
// set of key pairs
func ServerCredentialsFromKeyPairs(keys, caKeys authority.TLSKeyPair) (credentials.TransportCredentials, error) {
	cert, err := tls.X509KeyPair(keys.CertPEM, keys.KeyPEM)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caKeys.CertPEM); !ok {
		return nil, trace.BadParameter("failed to append CA to cert pool")
	}

	// Create the TLS credentials
	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    certPool,
	})
	return creds, nil
}

// DeleteCredentials deletes the package with RPC credentials from the specified package store
func DeleteCredentials(packages pack.PackageService) error {
	return packages.DeletePackage(loc.RPCSecrets)
}

// InitCredentials creates a package with RPC secrets in the specified package service
func InitCredentials(packages pack.PackageService) (*loc.Locator, error) {
	longLivedClient := true
	keys, err := GenerateAgentCredentials(nil, defaults.SystemAccountOrg, longLivedClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = createPackage(packages, loc.RPCSecrets, keys)
	if err != nil {
		return &loc.RPCSecrets, trace.Wrap(err)
	}

	return &loc.RPCSecrets, nil
}

// AgentAddr returns a complete agent address for specified address addr.
// If addr already contains a port, the address is returned unaltered,
// otherwise, a default RPC agent port is added
func AgentAddr(addr string) string {
	host, port := utils.SplitHostPort(addr, strconv.Itoa(defaults.GravityRPCAgentPort))
	return fmt.Sprintf("%v:%v", host, port)
}

// createPackage creates the secrets package pkg from archive in packages.
func createPackage(packages pack.PackageService, pkg loc.Locator, archive utils.TLSArchive) error {
	reader, err := utils.CreateTLSArchive(archive)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	err = packages.UpsertRepository(pkg.Repository, time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}

	labels := map[string]string{
		pack.PurposeLabel: pack.PurposeRPCCredentials,
	}
	_, err = packages.CreatePackage(pkg, reader, pack.WithLabels(labels))
	return trace.Wrap(err)
}

// upsertPackage creates or updates the secrets package pkg from archive in packages.
func upsertPackage(packages pack.PackageService, pkg loc.Locator, archive utils.TLSArchive) error {
	reader, err := utils.CreateTLSArchive(archive)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	err = packages.UpsertRepository(pkg.Repository, time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}

	labels := map[string]string{
		pack.PurposeLabel: pack.PurposeRPCCredentials,
	}
	_, err = packages.UpsertPackage(pkg, reader, pack.WithLabels(labels))
	return trace.Wrap(err)
}

func validateCertificateExpiration(pemBytes []byte, now time.Time) error {
	const tolerance = 30 * time.Second
	cert, err := tlsca.ParseCertificatePEM(pemBytes)
	if err != nil {
		return trace.Wrap(err)
	}
	if now.Add(-tolerance).Before(cert.NotBefore) {
		return trace.BadParameter("certificate is valid in the future").
			AddFields(trace.Fields{
				"now":        now,
				"not-before": cert.NotBefore,
			})
	}
	if now.Add(tolerance).After(cert.NotAfter) {
		return trace.BadParameter("certificate is valid in the past").
			AddFields(trace.Fields{
				"now":       now,
				"not-after": cert.NotAfter,
			})
	}
	return nil
}
