/*
Copyright 2018-2019 Gravitational, Inc.

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

package checks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/checks/autofix"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/dustin/go-humanize"
	"github.com/gravitational/satellite/agent/health"
	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/monitoring"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "checks")

// New creates a new checker for the specified list of servers using given
// set of server information payloads and the specified interface for
// running remote commands.
func New(config Config) (*checker, error) {
	if err := config.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &checker{Config: config}, nil
}

// Validate verifies the specified manifest against the host environment.
// Returns list of failed health probes.
func (r *ManifestValidator) Validate(ctx context.Context) (failedProbes []*agentpb.Probe, err error) {
	var errors []error
	requirements := r.Profile.Requirements
	if len(r.Mounts) != 0 {
		requirements = overrideMounts(requirements, r.Mounts)
	}
	failed, err := schema.ValidateRequirements(ctx, requirements, r.StateDir)
	if err != nil {
		errors = append(errors, trace.Wrap(err,
			"error validating profile requirements, see log file for details"))
	}
	failedProbes = append(failedProbes, failed...)

	dockerConfig := r.Manifest.Docker(r.Profile)
	if r.Docker != nil {
		dockerConfig.StorageDriver = r.Docker.StorageDriver
	}
	dockerSchema := schema.Docker{StorageDriver: dockerConfig.StorageDriver}
	failed, err = schema.ValidateDocker(ctx, dockerSchema, r.StateDir)
	if err != nil {
		errors = append(errors, trace.Wrap(err,
			"error validating docker requirements, see log file for details"))
	}
	failedProbes = append(failedProbes, failed...)

	failedProbes = append(failedProbes, schema.ValidateKubelet(ctx, r.Profile, r.Manifest)...)
	return failedProbes, trace.NewAggregate(errors...)
}

// ManifestValidator describes a manifest validator
type ManifestValidator struct {
	// Manifest specifies the manifest to validate against
	Manifest schema.Manifest
	// Profile specifies the node profile to validate against
	Profile schema.NodeProfile
	// StateDir specifies the state directory on the local node
	StateDir string
	// Docker specifies optional docker configuration.
	// If specified, overrides the system docker configuration
	Docker *storage.DockerConfig
	// Mounts specifies the mount overrides as name -> source path pairs
	Mounts map[string]string
}

// RunBasicChecks executes a set of additional health checks.
// Returns list of failed health probes.
func RunBasicChecks(ctx context.Context, options *validationpb.ValidateOptions) (failed []*agentpb.Probe) {
	var reporter health.Probes
	basicCheckers(options).Check(ctx, &reporter)

	for _, p := range reporter {
		if p.Status == agentpb.Probe_Failed {
			failed = append(failed, p)
		}
	}

	return failed
}

// LocalChecksRequest describes a request to run local pre-flight checks
type LocalChecksRequest struct {
	// Manifest is the application manifest to check against
	Manifest schema.Manifest
	// Role is the node profile name to check
	Role string
	// Options is additional validation options
	Options *validationpb.ValidateOptions
	// Docker specifies Docker configuration overrides (if any)
	Docker storage.DockerConfig
	// Mounts specidies optional mount overrides as name -> source path pairs
	Mounts map[string]string
	// AutoFix when set to true attempts to fix some common problems
	AutoFix bool
	// Progress is used to report information about auto-fixed problems
	utils.Progress
}

// CheckAndSetDefaults checks the request and sets some defaults
func (r *LocalChecksRequest) CheckAndSetDefaults() error {
	if r.Role == "" {
		return trace.BadParameter("role name is required")
	}
	if r.Progress == nil {
		r.Progress = utils.DiscardProgress
	}
	return nil
}

// LocalChecksResult describes the outcome of local checks execution
type LocalChecksResult struct {
	// Failed is a list of failed probes
	Failed []*agentpb.Probe
	// Fixed is a list of probes that failed but have been auto-fixed
	Fixed []*agentpb.Probe
	// Fixable is a list of probes that can be attempted to auto-fix
	Fixable []*agentpb.Probe
}

// GetFailed returns a list of all failed probes
func (r *LocalChecksResult) GetFailed() []*agentpb.Probe {
	return append(r.Failed, r.Fixable...)
}

// ValidateLocal runs checks on the local node and returns their outcome
func ValidateLocal(ctx context.Context, req LocalChecksRequest) (*LocalChecksResult, error) {
	if ifTestsDisabled() {
		log.Infof("Skipping local checks due to %v set.", constants.PreflightChecksOffEnvVar)
		return &LocalChecksResult{}, nil
	}

	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	profile, err := req.Manifest.NodeProfiles.ByName(req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	autofix.AutoloadModules(ctx, schema.DefaultKernelModules, req.Progress)

	dockerConfig := DockerConfigFromSchemaValue(req.Manifest.SystemDocker())
	OverrideDockerConfig(&dockerConfig, req.Docker)

	v := ManifestValidator{
		Manifest: req.Manifest,
		Profile:  *profile,
		StateDir: stateDir,
		Docker:   &dockerConfig,
		Mounts:   req.Mounts,
	}

	failedProbes, err := v.Validate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	failedProbes = append(failedProbes, RunBasicChecks(ctx, req.Options)...)
	if len(failedProbes) == 0 {
		return &LocalChecksResult{}, nil
	}

	if !req.AutoFix {
		failed, fixable := autofix.GetFixable(failedProbes)
		return &LocalChecksResult{
			Failed:  failed,
			Fixable: fixable,
		}, nil
	}

	// try to auto-fix some of the issues
	fixed, unfixed := autofix.Fix(ctx, failedProbes, req.Progress)
	return &LocalChecksResult{
		Failed: unfixed,
		Fixed:  fixed,
	}, nil
}

// RunLocalChecks performs all preflight checks for an application that can
// be run locally on the node
func RunLocalChecks(ctx context.Context, req LocalChecksRequest) error {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	result, err := ValidateLocal(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(result.GetFailed()) != 0 {
		return trace.BadParameter("The following pre-flight checks failed:\n%v",
			FormatFailedChecks(result.GetFailed()))
	}
	return nil
}

// FormatFailedChecks returns failed checks formatted as a list
func FormatFailedChecks(failed []*agentpb.Probe) string {
	if len(failed) == 0 {
		return ""
	}
	var buf bytes.Buffer
	for _, p := range failed {
		fmt.Fprintf(&buf, "\t[%v] %s\n", constants.FailureMark, formatProbe(*p))
	}
	return buf.String()
}

// OverrideDockerConfig updates given config with values from overrideConfig where necessary
func OverrideDockerConfig(config *storage.DockerConfig, overrideConfig storage.DockerConfig) {
	if overrideConfig.StorageDriver != "" {
		config.StorageDriver = overrideConfig.StorageDriver
	}
	if len(overrideConfig.Args) != 0 {
		config.Args = overrideConfig.Args
	}
}

// DockerConfigFromSchema converts the specified Docker schema to storage configuration format
func DockerConfigFromSchema(dockerSchema *schema.Docker) (config storage.DockerConfig) {
	if dockerSchema == nil {
		return config
	}
	return DockerConfigFromSchemaValue(*dockerSchema)
}

// DockerConfigFromSchemaValue converts the specified Docker schema to storage configuration format
func DockerConfigFromSchemaValue(dockerSchema schema.Docker) (config storage.DockerConfig) {
	return storage.DockerConfig{
		StorageDriver: dockerSchema.StorageDriver,
		Args:          dockerSchema.Args,
	}
}

// Checker defines a preflight checker interface.
type Checker interface {
	// Run runs a full set of checks on the nodes configured in the checker.
	Run(ctx context.Context) error
	// CheckNode executes single-node checks (such as CPU/RAM requirements,
	// disk space, etc) for the provided server.
	CheckNode(ctx context.Context, server Server) []*agentpb.Probe
	// CheckNodes executes multi-node checks (such as network reachability,
	// bandwidth, etc) on the provided set of servers.
	CheckNodes(ctx context.Context, servers []Server) []*agentpb.Probe
}

type checker struct {
	// Config is the checker configuration.
	Config
}

// Config represents the checker configuration.
type Config struct {
	// Remote is an interface for validating and executing commands on remote nodes.
	Remote Remote
	// Manifest is the cluster manifest the checker validates nodes against.
	Manifest schema.Manifest
	// Servers is a list of nodes for validation.
	Servers []Server
	// Requirements maps node roles to their validation requirements.
	Requirements map[string]Requirements
	// Features allows to turn certain checks off.
	Features
}

// check validates the checker configuration.
func (c Config) check() error {
	for _, server := range c.Servers {
		if _, exists := c.Requirements[server.Server.Role]; !exists {
			return trace.NotFound("no requirements for node profile %q",
				server.Server.Role)
		}
	}
	return nil
}

// Features controls which tests the checker will run
type Features struct {
	// TestBandwidth specifies whether the network bandwidth test should
	// be executed.
	TestBandwidth bool
	// TestPorts specifies whether the ports availability test should
	// be executed.
	TestPorts bool
}

// String return textual representation of this server object
func (r Server) String() string {
	return fmt.Sprintf("%v/%v", r.GetHostname(), r.AdvertiseIP)
}

// Server describes a remote node
type Server struct {
	// Server defines a remote node
	storage.Server
	// ServerInfo describes the remote node environment
	ServerInfo
}

// Run runs a full set of checks on the servers specified in r.servers
func (r *checker) Run(ctx context.Context) error {
	if ifTestsDisabled() {
		log.Infof("Skipping checks due to %q set.",
			constants.PreflightChecksOffEnvVar)
		return nil
	}

	var failed []*agentpb.Probe

	// check each server against its profile
	for _, server := range r.Servers {
		failed = append(failed, r.CheckNode(ctx, server)...)
	}

	// run checks that take all servers into account
	failed = append(failed, r.CheckNodes(ctx, r.Servers)...)

	if len(failed) != 0 {
		return trace.BadParameter("The following checks failed:\n%v",
			FormatFailedChecks(failed))
	}

	return nil
}

// CheckNode executes checks for the provided individual server.
func (r *checker) CheckNode(ctx context.Context, server Server) (failed []*agentpb.Probe) {
	if ifTestsDisabled() {
		log.Infof("Skipping single-node checks due to %q set.",
			constants.PreflightChecksOffEnvVar)
		return nil
	}

	requirements := r.Requirements[server.Server.Role]
	validateCtx, cancel := context.WithTimeout(ctx, defaults.AgentValidationTimeout)
	defer cancel()

	failed, err := r.Remote.Validate(validateCtx, server.AdvertiseIP, ValidateConfig{
		Manifest: r.Manifest,
		Profile:  server.Server.Role,
		Docker:   requirements.Docker,
	})
	if err != nil {
		log.WithError(err).Warn("Failed to validate remote node.")
		failed = append(failed, &agentpb.Probe{
			Detail: err.Error(),
			Error:  fmt.Sprintf("failed to validate node %v", server),
		})
	}

	err = checkServerProfile(server, requirements)
	if err != nil {
		log.WithError(err).Warn("Failed to validate profile requirements.")
		failed = append(failed, &agentpb.Probe{
			Detail: err.Error(),
			Error:  "failed to validate profile requirements",
		})
	}

	err = r.checkTempDir(ctx, server)
	if err != nil {
		log.WithError(err).Warn("Failed to validate temporary directory.")
		failed = append(failed, &agentpb.Probe{
			Detail: err.Error(),
			Error:  "failed to validate temporary directory",
		})
	}

	err = r.checkDisks(ctx, server)
	if err != nil {
		log.WithError(err).Warn("Failed to validate disk requirements.")
		failed = append(failed, &agentpb.Probe{
			Detail: err.Error(),
			Error:  "failed to validate disk requirements",
		})
	}

	return failed
}

// CheckNodes executes checks that take all provided servers into account.
func (r *checker) CheckNodes(ctx context.Context, servers []Server) (failed []*agentpb.Probe) {
	if ifTestsDisabled() {
		log.Infof("Skipping multi-node checks due to %q set.",
			constants.PreflightChecksOffEnvVar)
		return nil
	}

	err := checkSameOS(servers)
	if err != nil {
		log.WithError(err).Warn("Failed to validate same OS requirements.")
		failed = append(failed, &agentpb.Probe{
			Detail: err.Error(),
			Error:  "failed to validate same OS requirement",
		})
	}

	err = checkTime(time.Now().UTC(), servers)
	if err != nil {
		log.WithError(err).Warn("Failed to validate time drift requirements.")
		failed = append(failed, &agentpb.Probe{
			Detail: err.Error(),
			Error:  "failed to validate time drift requirement",
		})
	}

	if r.TestPorts {
		err = r.checkPorts(ctx, servers)
		if err != nil {
			log.WithError(err).Warn("Failed to validate port requirements.")
			failed = append(failed, &agentpb.Probe{
				Detail: err.Error(),
				Error:  "failed to validate port requirements",
			})
		}
	}

	if r.TestBandwidth {
		err = r.checkBandwidth(ctx, servers)
		if err != nil {
			log.WithError(err).Warn("Failed to validate bandwidth requirements.")
			failed = append(failed, &agentpb.Probe{
				Detail: err.Error(),
				Error:  "failed to validate network bandwidth requirements",
			})
		}
	}

	return failed
}

// checkDisks verifies that disk performance satisfies the profile requirements.
func (r *checker) checkDisks(ctx context.Context, server Server) error {
	requirements := r.Requirements[server.Server.Role]
	targets, err := r.collectTargets(ctx, server, requirements)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, target := range targets {
		var maxBps uint64

		// use the maximum throughput measured over a couple of tests
		for i := 0; i < 3; i++ {
			speed, err := r.checkServerDisk(ctx, server.Server, target.path)
			if err != nil {
				return trace.Wrap(err, "failed to sample disk performance at %v on %v",
					target.path, server.ServerInfo.GetHostname())
			}
			maxBps = utils.MaxInt64(speed, maxBps)
		}

		if maxBps < target.rate.BytesPerSecond() {
			return trace.BadParameter(
				"server %q disk I/O on %q is %v/s which is lower than required %v",
				server.ServerInfo.GetHostname(), target, humanize.Bytes(maxBps),
				target.rate.String())
		}

		log.Infof("Server %q passed disk I/O check on %v: %v/s.",
			server.ServerInfo.GetHostname(), target, humanize.Bytes(maxBps))
	}

	return nil
}

// checkServerDisk runs a simple disk performance test and returns the write speed in bytes per second
func (r *checker) checkServerDisk(ctx context.Context, server storage.Server, target string) (uint64, error) {
	var out bytes.Buffer

	// remove the testfile after the test
	defer func() {
		// testfile was created only on real filesystem
		if !strings.HasPrefix(target, "/dev") {
			err := r.Remote.Exec(ctx, server.AdvertiseIP, []string{"rm", target}, &out)
			if err != nil {
				log.WithField("output", out.String()).Warn("Failed to remove test file.")
			}
		}
	}()

	err := r.Remote.Exec(ctx, server.AdvertiseIP, []string{
		"dd", "if=/dev/zero", fmt.Sprintf("of=%v", target),
		"bs=100K", "count=1024", "conv=fdatasync"}, &out)
	if err != nil {
		log.WithFields(logrus.Fields{
			"server-ip": server.AdvertiseIP,
			"target":    target,
			"output":    out.String(),
		}).Warn("Failed to sample disk performance.")
		return 0, trace.Wrap(err, "failed to sample disk performance: %s", out.String())
	}

	speed, err := utils.ParseDDOutput(out.String())
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return speed, nil
}

// checkTempDir makes sure agents can create temporary files on servers
func (r *checker) checkTempDir(ctx context.Context, server Server) error {
	filename := filepath.Join(server.TempDir, fmt.Sprintf("tmpcheck.%v", uuid.New()))
	var out bytes.Buffer

	err := r.Remote.Exec(ctx, server.AdvertiseIP, []string{"touch", filename}, &out)
	if err != nil {
		log.WithFields(logrus.Fields{
			"filename":  filename,
			"server-ip": server.AdvertiseIP,
			"hostname":  server.ServerInfo.GetHostname(),
		}).Warn("Failed to create a test file.")
		return trace.BadParameter("failed to create a test file %v on %q: %v",
			filepath.Join(server.TempDir, filename), server.ServerInfo.GetHostname(), out.String())
	}

	err = r.Remote.Exec(ctx, server.AdvertiseIP, []string{"rm", filename}, &out)
	if err != nil {
		log.WithFields(logrus.Fields{
			"path":      filename,
			"server-ip": server.AdvertiseIP,
			"output":    out.String(),
		}).Warn("Failed to delete.")
	}

	log.Infof("Server %q passed temp directory check: %v.",
		server.ServerInfo.GetHostname(), server.TempDir)
	return nil
}

// checkPorts makes sure ports specified in profile are unoccupied and reachable
func (r *checker) checkPorts(ctx context.Context, servers []Server) error {
	req, err := constructPingPongRequest(servers, r.Requirements)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Ping pong request: %v.", req)

	if len(req) == 0 {
		log.Info("Empty ping pong request.")
		return nil
	}

	resp, err := r.Remote.CheckPorts(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Ping pong response: %v.", resp)

	if len(resp.Failures()) != 0 {
		return trace.BadParameter(strings.Join(resp.Failures(), ", "))
	}

	return nil
}

// checkBandwidth measures network bandwidth between servers and makes sure it satisfies
// the profile
func (r *checker) checkBandwidth(ctx context.Context, servers []Server) error {
	if len(servers) < 2 {
		return nil
	}

	req, err := constructBandwidthRequest(servers)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Bandwidth test request: %v.", req)

	resp, err := r.Remote.CheckBandwidth(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Bandwidth test response: %v.", resp)

	if len(resp.Failures()) != 0 {
		return trace.BadParameter("%v", strings.Join(resp.Failures(), ", "))
	}

	for addr, result := range resp {
		ip, _ := utils.SplitHostPort(addr, "")
		server, err := findServer(servers, ip)
		if err != nil {
			return trace.Wrap(err)
		}

		requirements := r.Requirements[server.Server.Role]
		transferRate := requirements.Network.MinTransferRate
		if result.BandwidthResult < transferRate.BytesPerSecond() {
			return trace.BadParameter(
				"server %q network bandwidth is %v/s which is lower than required %v",
				server.ServerInfo.GetHostname(),
				humanize.Bytes(result.BandwidthResult),
				transferRate.String())
		}

		log.Infof("Server %q network bandwidth: %v/s.",
			server.ServerInfo.GetHostname(), humanize.Bytes(result.BandwidthResult))
	}

	return nil
}

// collectTargets returns a list of targets (devices or existing filesystems)
// for the disk performance test
func (r *checker) collectTargets(ctx context.Context, server Server, requirements Requirements) ([]diskCheckTarget, error) {
	var targets []diskCheckTarget

	// Explicit system state directory disk performance target
	targets = append(targets, diskCheckTarget{
		path: filepath.Join(server.Server.StateDir(), "testfile"),
		rate: defaultTransferRate,
	})

	remote := &serverRemote{server, r.Remote}
	// check if there's a system device specified
	if path := getDevicePath(server.SystemState.Device.Name,
		storage.DeviceName(server.SystemDevice)); path != "" {
		filesystem, err := system.GetFilesystem(ctx, path, remote)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if filesystem == "" {
			targets = append(targets, diskCheckTarget{
				path: path,
				rate: defaultTransferRate,
			})
		}
	}
	// if no system device has been specified or it has a filesystem,
	// use the mount point for the test
	if len(targets) == 0 {
		fi, err := systeminfo.FilesystemForDir(server.ServerInfo, server.Server.StateDir())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		targets = append(targets, diskCheckTarget{
			path: filepath.Join(fi.Filesystem.DirName, "testfile"),
			rate: defaultTransferRate,
		})
	}

	// add all directories with their rates from the profile
	for _, volume := range requirements.Volumes {
		if volume.MinTransferRate == 0 {
			continue
		}
		fi, err := systeminfo.FilesystemForDir(server.ServerInfo, volume.Path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		targets = append(targets, diskCheckTarget{
			path: filepath.Join(fi.Filesystem.DirName, "testfile"),
			rate: volume.MinTransferRate,
		})
	}

	return targets, nil
}

func getDevicePath(devices ...storage.DeviceName) (path string) {
	for _, device := range devices {
		if device.Path() != "" {
			return device.Path()
		}
	}
	return ""
}

// diskCheckTarget combines attributes for a disk performance test
type diskCheckTarget struct {
	path string
	rate utils.TransferRate
}

// String implements fmt.Stringer
func (r diskCheckTarget) String() string {
	return fmt.Sprintf("disk(path=%v, rate=%v)", r.path, r.rate)
}

// checkServerProfile checks information for a single server collected by agent
// against its profile
func checkServerProfile(server Server, requirements Requirements) error {
	if requirements.CPU != nil {
		err := checkCPU(server.ServerInfo, *requirements.CPU)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if requirements.RAM != nil {
		err := checkRAM(server.ServerInfo, *requirements.RAM)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// checkCPU makes sure server's CPU count satisfies the profile
func checkCPU(info ServerInfo, cpu schema.CPU) error {
	if info.GetNumCPU() < uint(cpu.Min) {
		return trace.BadParameter("server %q has %v CPUs which is less than required minimum of %v",
			info.GetHostname(), info.GetNumCPU(), cpu.Min)
	}

	if cpu.Max != 0 && info.GetNumCPU() > uint(cpu.Max) {
		return trace.BadParameter("server %q has %v CPUs which exceeds configured maximum of %v",
			info.GetHostname(), info.GetNumCPU(), cpu.Max)
	}

	log.Infof("Server %q passed CPU check: %v.", info.GetHostname(), info.GetNumCPU())
	return nil
}

// checkRAM makes sure server's RAM amount satisfies the profile
func checkRAM(info ServerInfo, ram schema.RAM) error {
	if info.GetMemory().Total < ram.Min.Bytes() {
		return trace.BadParameter("server %q has %v RAM which is less than required minimum of %v",
			info.GetHostname(), humanize.Bytes(info.GetMemory().Total), ram.Min.String())
	}

	if ram.Max != 0 && info.GetMemory().Total > ram.Max.Bytes() {
		return trace.BadParameter("server %q has %v RAM which exceeds configured maximum of %v",
			info.GetHostname(), humanize.Bytes(info.GetMemory().Total), ram.Max.String())
	}

	log.Infof("Server %q passed RAM check: %v.", info.GetHostname(),
		humanize.Bytes(info.GetMemory().Total))
	return nil
}

// checkSameOS verifies the OS distribution requirement for the specified set of servers.
// The check will pass if all nodes in the cluster are based on the same OS distribution and major version.
// Variance in minor/patch versions is acceptable.
func checkSameOS(servers []Server) error {
	// distros maps distribution name to list of versions
	distros := make(map[string][]string)
	for _, server := range servers {
		info := server.GetOS()
		distros[info.ID] = append(distros[info.ID], info.Version)
	}
	if len(distros) != 1 {
		return trace.BadParameter("servers have different OS distributions: %v", formatKeysAsList(distros))
	}
	// Version verification is purposely simple and will compare the prefixes
	// up to to either the first '.' or end of line
	for _, versions := range distros {
		if !verifyCommonVersionPrefix(versions...) {
			return trace.BadParameter("servers have different OS versions: %v", formatAsList(distros))
		}
	}
	log.Infof("Servers passed check for same OS: %v.", formatAsList(distros))
	return nil
}

func verifyCommonVersionPrefix(versions ...string) bool {
	if len(versions) <= 1 {
		return true
	}
	for i := 0; i < len(versions)-1; i += 1 {
		if !strings.EqualFold(
			strings.Split(versions[i], ".")[0],
			strings.Split(versions[i+1], ".")[0]) {
			return false
		}
	}
	return true
}

func formatAsList(m map[string][]string) (result []string) {
	result = make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, fmt.Sprintf("%v (%v)", k, v))
	}
	return result
}

func formatKeysAsList(m map[string][]string) (result []string) {
	result = make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// checkTime checks if time it out of sync between servers
func checkTime(currentTime time.Time, servers []Server) error {
	// server can not be out of sync with itself
	if len(servers) < 2 {
		return nil
	}

	// use time of the first server as a base point in time comparison
	baseTime := currentServerTime(currentTime, servers[0].LocalTime, servers[0].ServerTime)
	for i := 1; i < len(servers); i++ {
		serverTime := currentServerTime(currentTime, servers[i].LocalTime, servers[i].ServerTime)
		delta := serverTime.Sub(baseTime)
		if delta < 0 {
			delta *= -1
		}
		if delta > defaults.MaxOutOfSyncTimeDelta {
			return trace.BadParameter(
				"servers %v and %v clocks are out of sync: %v and %v respectively, "+
					"sync the times on servers before install, e.g. using ntp",
				servers[0].GetHostname(),
				servers[i].GetHostname(),
				baseTime.Format(constants.HumanDateFormatMilli),
				serverTime.Format(constants.HumanDateFormatMilli))
		}
	}

	log.Infof("Servers %v passed time drift check.", servers)
	return nil
}

// currentServerTime calculates the current time on the server using
// local current time (t1), time of the last heartbeat (t2) and reported server time (t3)
// using the following formula:
// current server time = t3 + (t1 - t2)
func currentServerTime(currentTime, heartbeatTime, serverTime time.Time) time.Time {
	delta := currentTime.Sub(heartbeatTime)
	return serverTime.Add(delta)
}

func basicCheckers(options *validationpb.ValidateOptions) health.Checker {
	return monitoring.NewCompositeChecker(
		"local",
		[]health.Checker{
			monitoring.NewIPForwardChecker(),
			monitoring.NewBridgeNetfilterChecker(),
			monitoring.NewMayDetachMountsChecker(),
			monitoring.DefaultProcessChecker(),
			defaultPortChecker(options),
			monitoring.DefaultBootConfigParams(),
		},
	)
}

func defaultPortChecker(options *validationpb.ValidateOptions) health.Checker {
	vxlanPort := uint64(defaults.VxlanPort)
	if options != nil && options.VxlanPort != 0 {
		vxlanPort = uint64(options.VxlanPort)
	}

	var portRanges = []monitoring.PortRange{
		{Protocol: "tcp", From: 7496, To: 7496, Description: "serf (health check agents) peer to peer"},
		{Protocol: "tcp", From: 7373, To: 7373, Description: "serf (health check agents) peer to peer"},
		{Protocol: "tcp", From: 2379, To: 2380, Description: "etcd"},
		{Protocol: "tcp", From: 4001, To: 4001, Description: "etcd"},
		{Protocol: "tcp", From: 7001, To: 7001, Description: "etcd"},
		{Protocol: "tcp", From: 6443, To: 6443, Description: "kubernetes API server"},
		{Protocol: "tcp", From: 10248, To: 10255, Description: "kubernetes internal services range"},
		{Protocol: "tcp", From: 5000, To: 5000, Description: "docker registry"},
		{Protocol: "tcp", From: 3022, To: 3025, Description: "teleport internal SSH control panel"},
		{Protocol: "tcp", From: 3080, To: 3080, Description: "teleport Web UI"},
		{Protocol: "tcp", From: 3008, To: 3011, Description: "internal Gravity services"},
		{Protocol: "tcp", From: 32009, To: 32009, Description: "Gravity Hub control panel"},
		{Protocol: "tcp", From: 7575, To: 7575, Description: "Gravity RPC agent"},
		{Protocol: "udp", From: vxlanPort, To: vxlanPort, Description: "overlay network"},
	}

	dnsConfig := storage.DefaultDNSConfig
	if options != nil && len(options.DnsAddrs) != 0 {
		dnsConfig.Addrs = options.DnsAddrs
		if options.DnsPort != 0 {
			dnsConfig.Port = int(options.DnsPort)
		}
	}
	for _, addr := range dnsConfig.Addrs {
		portRanges = append(portRanges,
			monitoring.PortRange{
				Protocol:    "tcp",
				Description: "internal cluster DNS",
				From:        uint64(dnsConfig.Port),
				To:          uint64(dnsConfig.Port),
				ListenAddr:  addr,
			},
		)
	}

	return monitoring.NewPortChecker(portRanges...)
}

// constructPingPongRequest constructs a regular ping-pong game request
func constructPingPongRequest(servers []Server, requirements map[string]Requirements) (PingPongGame, error) {
	game := make(PingPongGame, len(servers))
	var listenServers []validationpb.Addr
	for _, server := range servers {
		profile := requirements[server.Server.Role]
		if len(profile.Network.Ports.TCP) == 0 && len(profile.Network.Ports.UDP) == 0 {
			continue
		}

		req := PingPongRequest{
			Duration: defaults.PingPongDuration,
			Mode:     ModePingPong,
		}
		for _, port := range profile.Network.Ports.TCP {
			listenServer := validationpb.Addr{
				Addr:    fmt.Sprintf("%v:%v", server.AdvertiseIP, port),
				Network: "tcp",
			}
			req.Listen = append(req.Listen, listenServer)
			listenServers = append(listenServers, listenServer)
		}
		for _, port := range profile.Network.Ports.UDP {
			listenServer := validationpb.Addr{
				Addr:    fmt.Sprintf("%v:%v", server.AdvertiseIP, port),
				Network: "udp",
			}
			req.Listen = append(req.Listen, listenServer)
			listenServers = append(listenServers, listenServer)
		}

		game[server.AdvertiseIP] = req
	}

	for ip, req := range game {
		req.Ping = listenServers
		game[ip] = req
	}

	return game, nil
}

// constructBandwidthRequest constructs a ping-pong game request for a bandwidth test
func constructBandwidthRequest(servers []Server) (PingPongGame, error) {
	// use up to defaults.BandwidthTestMaxServers servers for the test
	servers = servers[:utils.Min(len(servers), defaults.BandwidthTestMaxServers)]

	// construct a ping pong game
	game := make(PingPongGame, len(servers))
	for _, server := range servers {
		var remote []validationpb.Addr
		for _, other := range servers {
			if server.AdvertiseIP != other.AdvertiseIP {
				remote = append(remote, validationpb.Addr{
					Addr: other.AdvertiseIP,
				})
			}
		}
		game[server.AdvertiseIP] = PingPongRequest{
			Duration: defaults.BandwidthTestDuration,
			Listen: []validationpb.Addr{{
				Addr: server.AdvertiseIP,
			}},
			Ping: remote,
			Mode: ModeBandwidth,
		}
	}

	return game, nil
}

func findServer(servers []Server, addr string) (*Server, error) {
	for _, server := range servers {
		if server.AdvertiseIP == addr {
			return &server, nil
		}
	}
	return nil, trace.NotFound("server %v not found", addr)
}

func formatProbe(probe agentpb.Probe) string {
	if probe.Error != "" {
		if probe.Detail != "" {
			return fmt.Sprintf("%s (%s)", probe.Error, probe.Detail)
		}
		return probe.Error
	}
	return probe.Detail
}

func ifTestsDisabled() bool {
	envVar := os.Getenv(constants.PreflightChecksOffEnvVar)
	if envVar == "" {
		return false
	}
	disabled, _ := strconv.ParseBool(envVar)
	return disabled

}

func overrideMounts(requirements schema.Requirements, mounts map[string]string) schema.Requirements {
	var result []schema.Volume
	for _, volume := range requirements.Volumes {
		if path, ok := mounts[volume.Name]; !ok {
			result = append(result, volume)
		} else {
			v := volume
			v.Path = path
			result = append(result, v)
		}
	}
	requirements.Volumes = result
	return requirements
}

// RunStream executes the specified command on r.server.
// Implements utils.CommandRunner
func (r *serverRemote) RunStream(ctx context.Context, w io.Writer, args ...string) error {
	return trace.Wrap(r.remote.Exec(ctx, r.server.AdvertiseIP, args, w))
}

type serverRemote struct {
	server Server
	remote Remote
}

var (
	// defaultTransferRate defines default transfer rate requirement for some system volumes
	defaultTransferRate = utils.MustParseTransferRate(defaults.DiskTransferRate)
)
