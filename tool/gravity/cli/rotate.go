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
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func rotateRPCCredentials(env *localenv.LocalEnvironment, o rotateRPCCredsOptions) (err error) {
	clusterEnv, err := newClusterEnvironmentForRotate(env)
	if err != nil {
		return trace.Wrap(err)
	}
	archive, err := rpc.LoadCredentials(clusterEnv.Packages)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	now := time.Now()
	if o.show {
		if archive == nil {
			return trace.NotFound("no RPC credentials found. Run the command again without --show to generate.")
		}
		if err := dumpCertificates(archive); err != nil {
			log.WithError(err).Warn("Failed to output certificate expiration dates.")
		}
		env.Println("Validate certificates.")
		if err := rpc.ValidateCredentials(archive, now); err != nil {
			return trace.Wrap(err)
		}
		env.Println("Nothing to do.")
		return nil
	}
	if archive != nil {
		env.Println("Validate certificates.")
		err = rpc.ValidateCredentials(archive, now)
		if err == nil {
			env.Println("Nothing to do.")
			return nil
		}
	}
	env.Println("Generate new certificates.")
	_, err = rpc.UpsertCredentials(clusterEnv.Packages)
	if err != nil {
		return trace.Wrap(err)
	}
	env.Println("Restart cluster controller.")
	err = restartClusterControllerAndWait(env)
	if err != nil {
		return trace.Wrap(err)
	}
	env.Println("Done.")
	return nil
}

func dumpCertificates(archive utils.TLSArchive) error {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	defer w.Flush()
	fmt.Fprintln(w, "Certificate\tNot-Before\tNot-After")
	caKeyPair := archive[pb.CA]
	if err := dumpCertificateDates(caKeyPair.CertPEM, "CA", w); err != nil {
		return trace.Wrap(err)
	}
	clientKeyPair := archive[pb.Client]
	if err := dumpCertificateDates(clientKeyPair.CertPEM, "Client", w); err != nil {
		return trace.Wrap(err)
	}
	serverKeyPair := archive[pb.Server]
	if err := dumpCertificateDates(serverKeyPair.CertPEM, "Server", w); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func dumpCertificateDates(pemBytes []byte, prefix string, w io.Writer) error {
	cert, err := tlsca.ParseCertificatePEM(pemBytes)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(w, "%v\t%v\t%v\n", prefix, cert.NotBefore, cert.NotAfter)
	return nil
}

func restartClusterControllerAndWait(env *localenv.LocalEnvironment) error {
	cmd := exec.Command("kubectl", "delete", "pods", "--namespace", metav1.NamespaceSystem, "--selector", "app=gravity-site")
	out, err := cmd.CombinedOutput()
	log.WithField("cmd", cmd.Path).Info("Restart cluster controller.")
	if err != nil {
		return trace.Wrap(err, "failed to restart cluster controller: %s", out)
	}
	client := httplib.NewClient(httplib.WithInsecure(), httplib.WithLocalResolver(env.DNS.Addr()))
	b := utils.NewExponentialBackOff(defaults.ClusterStatusTimeout)
	return utils.RetryTransient(context.TODO(), b, func() error {
		return statusController(client)
	})
}

func statusController(client *http.Client) error {
	targetURL := defaults.GravityServiceURL + "/healthz"
	resp, err := client.Get(targetURL)
	if err != nil {
		return trace.Wrap(err, "failed to connect to %v", targetURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return trace.BadParameter("cluster is unhealthy")
}

func newClusterEnvironmentForRotate(env *localenv.LocalEnvironment) (*localenv.ClusterEnvironment, error) {
	nodeAddr, err := getLocalNodeAddr(env)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine local node advertise address")
	}
	serviceUser, err := getServiceUser(env)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine cluster service user")
	}
	return env.NewClusterEnvironment(
		localenv.WithNodeAddr(nodeAddr),
		localenv.WithServiceUser(*serviceUser),
	)
}

func getServiceUser(env *localenv.LocalEnvironment) (*systeminfo.User, error) {
	// Try the local state first
	user, err := env.Backend.GetServiceUser()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if user != nil {
		if serviceUser, err := systeminfo.FromOSUser(*user); err == nil {
			return serviceUser, nil
		} else {
			log.WithField("user", user).Warnf("Failed to convert system user: %v.", err)
			// Fall-through
		}
	}
	// Otherwise, use the cluster state for the lookup
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := clusterEnv.Operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return systeminfo.FromOSUser(cluster.ServiceUser)
}

func getLocalNodeAddr(env *localenv.LocalEnvironment) (nodeAddr string, err error) {
	// Try the local state first
	nodeAddr, err = env.Backend.GetNodeAddr()
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	if nodeAddr != "" {
		return nodeAddr, nil
	}
	// Otherwise, use the cluster server state for the lookup
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return "", trace.Wrap(err)
	}
	cluster, err := clusterEnv.Operator.GetLocalSite()
	if err != nil {
		return "", trace.Wrap(err)
	}
	server, err := ops.FindLocalServer(cluster.ClusterState)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return server.AdvertiseIP, nil
}

func rotateCertificates(env *localenv.LocalEnvironment, o rotateOptions) (err error) {
	if err := o.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
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

type rotateRPCCredsOptions struct {
	show bool
}

func (r *rotateOptions) checkAndSetDefaults() error {
	if r.clusterName == "" && r.caPath == "" {
		return trace.BadParameter("either cluster-name or ca-path needs to be specified")
	}
	return nil
}

type rotateOptions struct {
	// clusterName is the local cluster name
	clusterName string
	// validFor specifies validity duration for renewed certs
	validFor time.Duration
	// caPath is optional CA to use
	caPath string
}

const renewDuration = "26280h" // 3 years

var certNames = []string{
	constants.APIServerKeyPair,
	constants.ETCDKeyPair,
	constants.KubeletKeyPair,
	constants.ProxyKeyPair,
	constants.SchedulerKeyPair,
	constants.KubectlKeyPair,
}
