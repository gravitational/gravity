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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/configure/cstrings"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// RunKubernetesTests returns true if requested to run
// tests against running kubernetes. This mode requires
// kubeconfig to point to proper cluster with gravity running
func RunKubernetesTests() bool {
	testEnabled := os.Getenv(defaults.TestK8s)
	ok, _ := strconv.ParseBool(testEnabled)
	return ok
}

// RemoveNewlines removes newlines from string
func RemoveNewlines(s string) string {
	r := strings.NewReplacer("\r", " ", "\n", " ")
	return r.Replace(s)
}

// ParseAddrList parses a comma-separated list of addresses
func ParseAddrList(l string) ([]string, error) {
	return strings.Split(l, ","), nil
}

// ParseOpsCenterAddress parses OpsCenter address
func ParseOpsCenterAddress(in, defaultPort string) string {
	if in == "" {
		return ""
	}
	if !strings.Contains(in, "://") {
		in = "https://" + in
	}
	u, err := url.ParseRequestURI(in)
	if err != nil {
		return ""
	}
	if !strings.Contains(u.Host, ":") {
		u.Host = fmt.Sprintf("%v:%v", u.Host, defaultPort)
	}
	u.Path = ""
	return u.String()
}

// URLSplitHostPort extracts host name without port from URL
func URLSplitHostPort(in, defaultPort string) (string, string, error) {
	u, err := url.ParseRequestURI(in)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	host, port := SplitHostPort(u.Host, defaultPort)
	return host, port, nil
}

// ExtractHost returns only the host part from the provided address.
func ExtractHost(addr string) string {
	host, _ := SplitHostPort(addr, "")
	return host
}

// SplitHostPort extracts host name without port from host
func SplitHostPort(in, defaultPort string) (host string, port string) {
	parts := strings.SplitN(in, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], defaultPort
}

// EnsurePort makes sure that the provided address includes a port and adds
// the specified default one if it does not.
func EnsurePort(address, defaultPort string) string {
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}
	return net.JoinHostPort(address, defaultPort)
}

// EnsureScheme makes sure the provided URL contains http or https scheme and
// adds the specified default one if it does not.
func EnsureScheme(url, defaultScheme string) string {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	return fmt.Sprintf("%v://%v", defaultScheme, url)
}

// EnsurePortURL is like EnsurePort but for URLs.
func EnsurePortURL(url, defaultPort string) string {
	return ParseOpsCenterAddress(url, defaultPort)
}

// Hosts returns a list of hosts from the provided host:port addresses
func Hosts(addrs []string) (hosts []string) {
	for _, addr := range addrs {
		host, _ := SplitHostPort(addr, "")
		hosts = append(hosts, host)
	}
	return hosts
}

// ParseHostPort parses the provided address as host:port
func ParseHostPort(in string) (host string, port int32, err error) {
	host, portS, err := net.SplitHostPort(in)
	if err != nil {
		return "", 0, trace.Wrap(err)
	}
	portI, err := strconv.ParseInt(portS, 0, 32)
	if err != nil {
		return "", 0, trace.Wrap(err)
	}
	return host, int32(portI), nil
}

// URLHostname returns hostname without port for given URL address
func URLHostname(address string) (string, error) {
	targetURL, err := url.ParseRequestURI(address)
	if err != nil {
		return "", trace.Wrap(err, "failed parsing url %v", address)
	}

	if !strings.Contains(targetURL.Host, ":") {
		return targetURL.Host, nil
	}

	hostname, _, err := net.SplitHostPort(targetURL.Host)
	if err != nil {
		return "", trace.Wrap(err, "failed target host %v", targetURL.Host)
	}
	return hostname, nil
}

// ParseLabels parses a string like "a=b,c=d" as a map
func ParseLabels(labelsS string) map[string]string {
	labels := map[string]string{}
	for _, l := range strings.Split(labelsS, ",") {
		kv := strings.Split(l, "=")
		if len(kv) != 2 {
			continue
		}
		labels[kv[0]] = kv[1]
	}
	return labels
}

// DockerInfo is structured information returned by docker info
type DockerInfo struct {
	ServerVersion string
	StorageDriver string
}

// ParseDockerInfo parses output produced by `docker info` command
func ParseDockerInfo(r io.Reader) (*DockerInfo, error) {
	scanner := bufio.NewScanner(r)
	const driver = "Storage Driver:"
	const serverVersion = "Server Version:"
	var info DockerInfo
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, driver) {
			info.StorageDriver = strings.TrimSpace(strings.TrimPrefix(line, driver))
		}
		if strings.HasPrefix(line, serverVersion) {
			info.ServerVersion = strings.TrimSpace(strings.TrimPrefix(line, serverVersion))
		}
	}
	if info.ServerVersion == "" {
		return nil, trace.BadParameter("failed to extract version")
	}
	if info.StorageDriver == "" {
		return nil, trace.BadParameter("failed to extract storage driver")
	}
	return &info, nil
}

// ParsePorts parses the provided string specifying one or multiple ports or port ranges
// in the following form:
//
//   "80, 8081, 8001-8003"
//
// and translates it to an int slice with these ports
func ParsePorts(ranges string) ([]int, error) {
	var ports []int
	for _, port := range strings.Split(ranges, ",") {
		parts := strings.Split(port, "-")
		if len(parts) != 2 {
			// this must be a single port
			portInt, err := strconv.Atoi(strings.TrimSpace(port))
			if err != nil {
				return nil, trace.BadParameter("port must be integer, got: %v", port)
			}
			ports = append(ports, portInt)
		} else {
			// this is a port range, collect all ports in-between
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, trace.BadParameter("port must be integer, got: %v", parts[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, trace.BadParameter("port must be integer, got: %v", parts[1])
			}
			for i := start; i <= end; i++ {
				ports = append(ports, i)
			}
		}
	}
	return ports, nil
}

// ParseDDOutput parses the output of "dd" command and returns the reported
// speed in bytes per second.
//
// Example output:
//
// $ dd if=/dev/zero of=/tmp/testfile bs=1G count=1
// 1+0 records in
// 1+0 records out
// 1073741824 bytes (1.1 GB) copied, 4.52455 s, 237 MB/s
func ParseDDOutput(output string) (uint64, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		return 0, trace.BadParameter("expected 3 lines but got %v:\n%v", len(lines), output)
	}

	// 1073741824 bytes (1.1 GB) copied, 4.52455 s, 237 MB/s
	// 1073741824 bytes (1,1 GB, 1,0 GiB) copied, 4,53701 s, 237 MB/s
	testResults := lines[2]
	match := speedRe.FindStringSubmatch(testResults)
	if len(match) != 2 {
		return 0, trace.BadParameter("failed to match speed value (e.g. 237 MB/s) in %q", testResults)
	}

	// Support comma-formatted floats - depending on selected locale
	speedValue := strings.TrimSpace(strings.Replace(match[1], ",", ".", 1))
	value, err := strconv.ParseFloat(speedValue, 64)
	if err != nil {
		return 0, trace.Wrap(err, "failed to parse speed value as a float: %q", speedValue)
	}

	units := strings.TrimSpace(strings.TrimPrefix(match[0], match[1]))
	switch units {
	case "kB/s":
		return uint64(value * 1000), nil
	case "MB/s":
		return uint64(value * 1000 * 1000), nil
	case "GB/s":
		return uint64(value * 1000 * 1000 * 1000), nil
	default:
		return 0, trace.BadParameter("expected units (one of kB/s, MB/s, GB/s) but got %q", units)
	}
}

// User describes a system user as found in a passwd file.
// Adopted from https://raw.githubusercontent.com/opencontainers/runc/master/libcontainer/user/{user,lookup}.go
// See: man 5 passwd
type User struct {
	Name  string
	Pass  string
	Uid   int //nolint:stylecheck,revive
	Gid   int //nolint:stylecheck,revive
	Gecos string
	Home  string
	Shell string
}

// ParsePasswd interprets the specified passwd file as a list of users
func ParsePasswd(passwd io.Reader) (users []User, err error) {
	s := bufio.NewScanner(passwd)

	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, trace.Wrap(err)
		}

		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}

		// see: man 5 passwd
		//  name:password:UID:GID:GECOS:directory:shell
		// Name:Pass:Uid:Gid:Gecos:Home:Shell
		//  root:x:0:0:root:/root:/bin/bash
		//  adm:x:3:4:adm:/var/adm:/bin/false
		var user User
		err := parseLine(line, &user.Name, &user.Pass, &user.Uid, &user.Gid, &user.Gecos, &user.Home, &user.Shell)
		if err != nil {
			return nil, trace.Wrap(err, "invalid passwd entry: %s", line)
		}

		users = append(users, user)
	}

	return users, nil
}

// GetPasswd returns the reader to the contents of the passwd file
func GetPasswd() (io.ReadCloser, error) {
	return os.Open(unixPasswdPath)
}

func parseLine(line string, values ...interface{}) error {
	parts := strings.Split(line, ":")
	for i, part := range parts {
		// Ignore cases where we don't have enough fields to populate the arguments.
		// Some configuration files like to misbehave.
		if len(values) <= i {
			break
		}

		// Use the type of the argument to figure out how to parse it, scanf() style.
		// This is legit.
		switch argType := values[i].(type) {
		case *string:
			*argType = part
		case *int:
			// "numbers", with conversion errors ignored because of some misbehaving configuration files.
			*argType, _ = strconv.Atoi(part)
		case *[]string:
			// Comma-separated lists.
			if part != "" {
				*argType = strings.Split(part, ",")
			} else {
				*argType = []string{}
			}
		default:
			return trace.BadParameter("invalid argument type (%T)", argType)
		}
	}
	return nil
}

// ParseSystemdVersion parses the output of "systemctl --version" command
// and returns systemd version number. The output looks like this:
//   systemd 219
//   +PAM +AUDIT +SELINUX +IMA -APPARMOR +SMACK +SYSVINIT +UTMP +LIBCRYPTSETUP +GCRYPT +GNUTLS +ACL +XZ -LZ4 -SECCOMP +BLKID +ELFUTILS +KMOD +IDN
func ParseSystemdVersion(out string) (int, error) {
	out = strings.TrimSpace(out)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		return 0, trace.BadParameter("unexpected systemd version output: %q", out)
	}
	parts := strings.Split(lines[0], " ")
	if len(parts) != 2 {
		return 0, trace.BadParameter("unexpected systemd version output: %q", out)
	}
	version, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return version, nil
}

// Unix-specific path to the passwd formatted file.
const unixPasswdPath = "/etc/passwd"

var speedRe = regexp.MustCompile(`(\d+(?:[.,]\d+)?) \w+/s$`)

// ParseHostOverride parses DNS host override in the format <host>/<ip>
func ParseHostOverride(override string) (domain, ip string, err error) {
	parts := strings.Split(override, "/")
	if len(parts) != 2 {
		return "", "", trace.BadParameter("invalid DNS host override format %q, expected format: <host>/<ip>",
			override)
	}
	domain, ip = parts[0], parts[1]
	if !cstrings.IsValidDomainName(domain) {
		return "", "", trace.BadParameter("%q is not a valid domain name", domain)
	}
	if net.ParseIP(ip) == nil {
		return "", "", trace.BadParameter("%q is not a valid IP address", ip)
	}
	return domain, ip, nil
}

// ParseZoneOverride parses DNS zone override in the format <host>/<ip> or
// <host>/<ip>:<port>
func ParseZoneOverride(override string) (zone, nameserver string, err error) {
	parts := strings.Split(override, "/")
	if len(parts) != 2 {
		return "", "", trace.BadParameter("invalid DNS zone override format %q, expected <zone>/<ip>[:<port>]",
			override)
	}
	zone, nameserver = parts[0], parts[1]
	if !cstrings.IsValidDomainName(zone) {
		return "", "", trace.BadParameter("%q is not a valid domain name", zone)
	}
	// see if it's just an IP address
	if net.ParseIP(nameserver) != nil {
		return zone, nameserver, nil
	}
	// otherwise it includes port
	host, portS, err := net.SplitHostPort(nameserver)
	if err != nil {
		return "", "", trace.Wrap(err, "expected nameserver as <ip> or <ip>:<port>, got: %q", nameserver)
	}
	// host must be a valid IP address
	if net.ParseIP(host) == nil {
		return "", "", trace.BadParameter("%q is not a valid IP address", host)
	}
	// port must be numeric and in the correct range
	port, err := strconv.Atoi(portS)
	if err != nil {
		return "", "", trace.BadParameter("expected numeric port, got: %q", portS)
	}
	if port < 1 || port > 65535 {
		return "", "", trace.BadParameter("invalid port: %q", port)
	}
	return zone, nameserver, nil
}

// ToUnknownResource converts the provided resource to a generic resource type
func ToUnknownResource(resource teleservices.Resource) (*teleservices.UnknownResource, error) {
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var unknown teleservices.UnknownResource
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer(data), defaults.DecoderBufferSize).Decode(&unknown)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &unknown, nil
}

// ParseProxyAddr parses proxy address in the format "host:webPort,sshPort"
//
// If web/SSH ports are missing the provided defaults are used.
func ParseProxyAddr(proxyAddr, defaultWebPort, defaultSSHPort string) (host string, webPort string, sshPort string, err error) {
	host, port, err := net.SplitHostPort(proxyAddr)
	if err != nil {
		return proxyAddr, defaultWebPort, defaultSSHPort, nil
	}

	parts := strings.Split(port, ",")

	switch {

	case len(parts) == 0: // default ports for both the SSH and HTTP proxy
		return host, defaultWebPort, defaultSSHPort, nil

	case len(parts) == 1: // user defined HTTP proxy port, default SSH proxy port
		return host, parts[0], defaultSSHPort, nil

	case len(parts) == 2: // user defined HTTP and SSH proxy ports
		return host, parts[0], parts[1], nil
	}

	return "", "", "", trace.BadParameter("unable to parse port: %v", port)
}

// ParseBoolFlag extracts boolean parameter of the specified name from the
// provided request's query string, or returns default.
func ParseBoolFlag(r *http.Request, name string, def bool) (bool, error) {
	sValue := r.URL.Query().Get(name)
	if sValue == "" {
		return def, nil
	}
	bValue, err := strconv.ParseBool(sValue)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return bValue, nil
}

// ParseProxy parses the provided HTTP(-S) proxy address and returns it in
// the parsed URL form. The proxy value is expected to be a complete URL
// that includes a scheme (http, https or socks5).
//
// This function is loosely based on Go's proxy parsing method:
//
// https://github.com/golang/go/blob/release-branch.go1.15/src/vendor/golang.org/x/net/http/httpproxy/proxy.go#L149-L170
func ParseProxy(proxy string) (*url.URL, error) {
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse proxy address %q", proxy)
	}
	if !StringInSlice([]string{"http", "https", "socks5"}, proxyURL.Scheme) {
		return nil, trace.BadParameter("proxy address %q must include a valid scheme (http, https or socks5)", proxy)
	}
	return proxyURL, nil
}
