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

package server

import (
	"crypto/tls"
	"crypto/x509"

	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context" // TODO: remove in go1.9
	"google.golang.org/grpc/credentials"
	. "gopkg.in/check.v1" //nolint:revive,stylecheck // TODO: tests will be rewritten to use testify
)

// NewTestPeer creates a new peer instance for tests
func NewTestPeer(c *C, config PeerConfig, serverAddr string,
	cmd commandExecutor, sysinfo TestSystemInfo) *PeerServer {
	if config.Credentials.IsEmpty() {
		config.Credentials = TestCredentials(c)
	}
	if config.commandExecutor == nil {
		config.commandExecutor = cmd
	}
	if config.systemInfo == nil {
		config.systemInfo = sysinfo
	}
	peer, err := NewPeer(config, serverAddr)
	c.Assert(err, IsNil)

	return peer
}

// TestCredentials returns credentials for tests
func TestCredentials(c *C) Credentials {
	clientCreds := TestClientCredentials(c)
	creds := TestServerCredentials(c)
	return Credentials{Client: clientCreds, Server: creds}
}

// TestClientCredentials returns client credentials for tests
func TestClientCredentials(c *C) credentials.TransportCredentials {
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(certFile) {
		c.Error("failed to append certificates")
	}
	return credentials.NewTLS(&tls.Config{
		ServerName: "agent",
		RootCAs:    cp,
		MinVersion: tls.VersionTLS12,
	})
}

// TestServerCredentials returns server credentials for tests
func TestServerCredentials(c *C) credentials.TransportCredentials {
	cert, err := tls.X509KeyPair(certFile, keyFile)
	c.Assert(err, IsNil)
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
}

// nolint:errcheck
func (r TestCommand) exec(ctx context.Context, stream pb.OutgoingMessageStream, req pb.CommandArgs, log log.FieldLogger) error {
	stream.Send(&pb.Message{Element: &pb.Message_ExecStarted{ExecStarted: &pb.ExecStarted{Seq: 1, Args: req.Args}}})
	stream.Send(&pb.Message{Element: &pb.Message_ExecOutput{ExecOutput: &pb.ExecOutput{Data: []byte(r.output)}}})
	stream.Send(&pb.Message{Element: &pb.Message_ExecCompleted{ExecCompleted: &pb.ExecCompleted{Seq: 1, ExitCode: 0}}})
	return nil
}

// NewTestCommand returns a new instance of command executor
// serving the specified output
func NewTestCommand(output string) TestCommand {
	return TestCommand{output}
}

type TestCommand struct {
	output string
}

// NewTestSystemInfo returns a new test system info
func NewTestSystemInfo() TestSystemInfo {
	sysinfo := storage.NewSystemInfo(storage.SystemSpecV2{})
	return TestSystemInfo(*sysinfo)
}

func (r TestSystemInfo) getSystemInfo() (storage.System, error) {
	return (*storage.SystemV2)(&r), nil
}

// TestSystemInfo is an alias for system info for use in tests.
// Implements systemInfo
type TestSystemInfo storage.SystemV2

// 1. openssl genrsa -out server.key 2048
var keyFile = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAw6o84cRSm1w0GN/z8Vg03G1dJBeD7seTXPIfoxg/XiS+mQEz
K8gg5K0HHvvtn6eT0kiZ1tTK8eynOaziD99dI1HB+1Ly2miaVMJdBNMoKKRi8wqo
eLWXcQ2zBize/iQe7FbuyyxGTeZiqnZLI5qET7vUHz5ZATnjNs3VXsjNPidroROT
xpAIPHwwCFfOKq/aD7zHVPHBs36r6hiKi9j3PdECy+l52cwl5/xxuVUyTAO0KuJD
e+SkkSB1ahhjdRCJD3hgg4yJrQqgwQOnEot/EXcF8/6T6C33qNLvLCsHJ+xvCsp4
n6dO/FBw2k8QEYSBG6tCam7zQ/KG7xiqnVCqKQIDAQABAoIBAAKwo3ejIFOcd+bj
pVHrGYbyRfaKEDlHKyJ6/a5bVfuwW6J03sQ6UyFxs4hchE7OmfypHNxUPpoG2+Gy
G8WGF5y1sgoHgOk5yO1jYq3/TS0J3YZj3h8SuAtI2e46zbIGwxoSs+O9LxZBFZgs
WioaJLmH5omrbMPUjOgi+kz8S74P/ID9cIyvSLfXA1tBqaLZ0qvzg5fbO82Pv24T
DxpsqMZcI1Kms9vDk5kWk69ECQK1fm7VHdWW/nHNDWItyQ5dBdbtGhd5fo/X5VbY
IQTK7LYeRwEPA7PqZKzP3qEh5O1uYHE/HnXzZ4nBM61r/SQcWfwhOVYt7RXqA/fz
SyhTORUCgYEA8yu43WqeEZ/t15MOyfOT/GkSfxBaseZKHYqn5YDsHyewMKqYa14e
OKAS68XUBUE61fS1Pe1/gvDGQlmfw1tlt7t6FxtCdK8amCK+T2vxCBqQQYbGcJWE
ZlUlOdM0wOB2+X2ISKhHfPA8vi/G8ye1gUHhxfhftlr4w9MRUhQyqgsCgYEAzfzm
WbKm1sEAObIVVeYAcOTWKuW9hBGGGn+xF2jBUNXwm+vkqKHpxnNVjoUO4K8m39Lb
2wU1rA4rRAvgjLy8F4W7guv1Zrmd3lteGbulHk3ZGCYui542nKI6Ogj5GUtZpodq
neeAWzxz5RBYOClQQDYRznGzY5jSlfFVvBbKERsCgYAXe7xxnY9AUnqMnAYMmLpM
4PTJUpH/pia4LaDDOC0VYSbRvFfV3pP6kfLh1AwCqeb9rJEoNtxej9QFqlQUcKol
ETTcMGS9kf92e7x3PQxc5PvTaCmXy8iqfUSIDg6FJeg3ddkIcz/cH/Mtxr1m1Ani
PrOIA9FycdyeRK7ih1LROwKBgEi1dSXCPsvdElRLPOa2Kf+vdr1rnKqqeNiPrBXk
PyBmc+jFqk+v31HSUifdZbP/f0xQJJS50QkrczAwtRFYaVgwN1DuMxAQgt4DCEMz
DgSVXAT/LTzRGtvNE5p6olrAUyPJ9uNH3PHXc90uGMWyJ4aSz1Q8pCKKxgJxTl72
+FpzAoGAamIJ0FspmUEbLO+fLFMyjL6G/8sePAh4ul/kHD5i5vxhoY71A2tOMQ4p
NrEBphzjYVILvfDrpbx+3iWiSvbIGY9pM1yYIA7VqqX1Vn0+OOYcCNz9Fp7SwRxK
f5d6Jt9O4SfwGQZ2yTL3bzwkmJl493PRVGERIeMZ8fTFyBEKa2g=
-----END RSA PRIVATE KEY-----`)

// san.cnf:
// [req]
// req_extensions=req_ext
// prompt=no
// distinguished_name=dn
// [dn]
// C=US
// ST=Test
// L=Test
// O=Acme LLC
// CN=agent
// [req_ext]
// subjectAltName=@alt_names
// [alt_names]
// DNS.1=agent
// IP.1=127.0.0.1
// 2. openssl req -new -out server.csr -key server.key -config san.cnf
// 3. openssl x509 -req -days 3650 -in server.csr -signkey server.key -out server.crt -extensions req_ext -extfile san.cnf
var certFile = []byte(`-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIJANpmxTqLQ5HBMA0GCSqGSIb3DQEBCwUAME4xCzAJBgNV
BAYTAlVTMQ0wCwYDVQQIDARUZXN0MQ0wCwYDVQQHDARUZXN0MREwDwYDVQQKDAhB
Y21lIExMQzEOMAwGA1UEAwwFYWdlbnQwHhcNMTcxMjA4MTY1NDIzWhcNMjcxMjA2
MTY1NDIzWjBOMQswCQYDVQQGEwJVUzENMAsGA1UECAwEVGVzdDENMAsGA1UEBwwE
VGVzdDERMA8GA1UECgwIQWNtZSBMTEMxDjAMBgNVBAMMBWFnZW50MIIBIjANBgkq
hkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAw6o84cRSm1w0GN/z8Vg03G1dJBeD7seT
XPIfoxg/XiS+mQEzK8gg5K0HHvvtn6eT0kiZ1tTK8eynOaziD99dI1HB+1Ly2mia
VMJdBNMoKKRi8wqoeLWXcQ2zBize/iQe7FbuyyxGTeZiqnZLI5qET7vUHz5ZATnj
Ns3VXsjNPidroROTxpAIPHwwCFfOKq/aD7zHVPHBs36r6hiKi9j3PdECy+l52cwl
5/xxuVUyTAO0KuJDe+SkkSB1ahhjdRCJD3hgg4yJrQqgwQOnEot/EXcF8/6T6C33
qNLvLCsHJ+xvCsp4n6dO/FBw2k8QEYSBG6tCam7zQ/KG7xiqnVCqKQIDAQABoxow
GDAWBgNVHREEDzANggVhZ2VudIcEfwAAATANBgkqhkiG9w0BAQsFAAOCAQEAIiff
AjmdIl4x7adKn/ksqZs8cJtYYm3ufONOAWVidXVOZVK7Kj/Z9toLowW3RzxQbUo2
Qp2Vvp+XQE6z/BInpval1xwfpslCmq8FqJFhXmMuDFBuqusre5xs/LKNpa1Id5b2
Sp1xLEgKk20klHYukQo1bc12+zVD0yDszQD92b02KMrYM+1XX/FAwn6vpXFioT2T
q376XKXVpzwJBZ4w/+XavzeB0NfEo6oOQWyNIjCTCimV51pGNWoN6r4+v4xPz6VM
Xwdh8ncprPj+0H98KE2Y8J4c8u1kUcqe15/u6fNqpqn9VM8OToGfg3BSNlK8m8ZC
5p/3duIGPy8cYkQrcg==
-----END CERTIFICATE-----`)
