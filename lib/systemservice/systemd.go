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

package systemservice

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "systemd")

type systemdManager struct{}

const (
	systemdUnitFileTemplate = `[Unit]
Description={{.Description}}
{{with .Dependencies}}
{{if .Requires}}Requires={{.Requires}}{{end}}
{{if .After}}After={{.After}}{{end}}
{{if .Before}}Before={{.Before}}{{end}}
{{end}}
{{if .ConditionPathExists}}ConditionPathExists={{.ConditionPathExists}}{{end}}

[Service]
TimeoutStartSec={{.Timeout}}
{{if .Type}}Type={{.Type}}{{end}}
{{if .User}}User={{.User}}{{end}}
ExecStart={{.StartCommand}}{{range .StartPreCommands}}
ExecStartPre={{.}}{{end}}
{{if .StartPostCommand}}ExecStartPost={{.StartPostCommand}}{{end}}
{{if .StopCommand}}ExecStop={{.StopCommand}}{{end}}
{{if .StopPostCommand}}ExecStopPost={{.StopPostCommand}}{{end}}
{{if .LimitNoFile}}LimitNOFILE={{.LimitNoFile}}{{end}}
{{if .KillMode}}KillMode={{.KillMode}}{{end}}
{{if .KillSignal}}KillSignal={{.KillSignal}}{{end}}
{{if .Restart}}Restart={{.Restart}}{{end}}
{{if .TimeoutStopSec}}TimeoutStopSec={{.TimeoutStopSec}}{{end}}
{{if .RestartSec}}RestartSec={{.RestartSec}}{{end}}
{{if .RemainAfterExit}}RemainAfterExit=yes{{end}}
{{if .RestartPreventExitStatus}}RestartPreventExitStatus={{.RestartPreventExitStatus}}{{end}}
{{if .SuccessExitStatus}}SuccessExitStatus={{.SuccessExitStatus}}{{end}}
{{if .WorkingDirectory}}WorkingDirectory={{.WorkingDirectory}}{{end}}
{{range $k, $v := .Environment}}Environment={{$k}}={{$v}}
{{end}}
{{if .TasksMax}}TasksMax={{.TasksMax}}{{end}}

{{if .WantedBy}}
[Install]
WantedBy={{.WantedBy}}
{{end}}
`

	systemdServiceDelimiter = "__"
)

var serviceUnitTemplate = template.Must(template.New("").Parse(systemdUnitFileTemplate))

var mountUnitTemplate = template.Must(template.New("mount").
	Funcs(template.FuncMap{"join": strings.Join}).
	Parse(`
[Mount]
What={{.What}}
Where={{.Where}}
{{if .Type}}Type={{.Type}}{{end}}
{{if .Options}}Options={{join .Options ","}}{{end}}
{{if .TimeoutSec}}TimeoutSec={{.TimeoutSec}}{{end}}

[Install]
WantedBy=local-fs.target
`))

func newSystemdUnit(pkg loc.Locator) *systemdUnit {
	return &systemdUnit{pkg: pkg}
}

type systemdUnit struct {
	pkg loc.Locator
}

func parseUnit(unit string) *loc.Locator {
	unit = strings.TrimSuffix(unit, ServiceSuffix)
	parts := strings.Split(unit, systemdServiceDelimiter)
	if len(parts) != 4 {
		return nil
	}
	loc, err := loc.NewLocator(parts[1], parts[2], parts[3])
	if err != nil {

		return nil
	}
	return loc
}

func (u *systemdUnit) serviceName() string {
	return strings.Join([]string{
		servicePrefix, u.pkg.Repository, u.pkg.Name, u.pkg.Version},
		systemdServiceDelimiter) + ServiceSuffix
}

func (u *systemdUnit) servicePath() string {
	return unitPath(u.serviceName())
}

func (s *systemdManager) installService(service serviceTemplate, req NewServiceRequest) error {
	if service.Environment == nil {
		service.Environment = make(map[string]string)
	}
	if _, ok := service.Environment[defaults.PathEnv]; !ok {
		service.Environment[defaults.PathEnv] = defaults.PathEnvVal
	}
	f, err := os.Create(unitPath(req.Name))
	if err != nil {
		return trace.Wrap(err,
			"error creating systemd unit file at %v", unitPath(req.Name))
	}
	defer f.Close()

	err = serviceUnitTemplate.Execute(f, service)
	if err != nil {
		return trace.Wrap(err, "error rendering template")
	}

	if req.ReloadConfiguration {
		if out, err := invokeSystemctl("daemon-reload"); err != nil {
			return trace.Wrap(err, "failed to reload manager's configuration").AddField("output", out)
		}
	}

	if err := s.EnableService(req.Name); err != nil {
		return trace.Wrap(err, "error enabling the service")
	}

	if err := s.StartService(service.Name, req.NoBlock); err != nil {
		return trace.Wrap(err, "error starting the service")
	}

	return nil
}

func (s *systemdManager) installMountService(service mountServiceTemplate, noBlock bool) error {
	servicePath := filepath.Join(defaults.SystemUnitDir, SystemdNameEscape(service.Name))
	f, err := os.Create(servicePath)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"error creating mount systemd unit file at %v", servicePath)
	}
	defer f.Close()

	err = mountUnitTemplate.Execute(f, service)
	if err != nil {
		return trace.Wrap(err, "error rendering template")
	}

	if err := s.EnableService(service.Name); err != nil {
		return trace.Wrap(err, "error enabling the service")
	}

	if err := s.StartService(service.Name, noBlock); err != nil {
		return trace.Wrap(err, "error starting the service")
	}

	return nil
}

// InstallPackageService installs gravity service implemented as a gravity package command
func (s *systemdManager) InstallPackageService(req NewPackageServiceRequest) error {
	unit := newSystemdUnit(req.Package)

	if req.WantedBy == "" {
		req.WantedBy = defaults.SystemServiceWantedBy
	}

	if req.RestartSec == 0 {
		req.RestartSec = defaults.SystemServiceRestartSec
	}

	if req.TasksMax == "" && s.supportsTasksAccounting() {
		req.TasksMax = defaults.SystemServiceTasksMax
	}

	req.StartCommand = fmt.Sprintf("%v package command %v %v %v", req.GravityPath, req.StartCommand, req.Package, req.ConfigPackage)
	if req.StopCommand != "" {
		req.StopCommand = fmt.Sprintf("%v package command %v %v %v", req.GravityPath, req.StopCommand, req.Package, req.ConfigPackage)
	}

	template := serviceTemplate{
		Name:        unit.serviceName(),
		ServiceSpec: req.ServiceSpec,
		Description: fmt.Sprintf("Auto-generated service for the %v package", req.Package),
	}

	return trace.Wrap(s.installService(template, NewServiceRequest{
		ServiceSpec: req.ServiceSpec,
		Name:        unit.serviceName(),
		NoBlock:     req.NoBlock,
	}))
}

// UninstallPackageService uninstalls gravity service implemented as a gravity package command
func (s *systemdManager) UninstallPackageService(pkg loc.Locator) error {
	return trace.Wrap(s.UninstallService(UninstallServiceRequest{
		Name: newSystemdUnit(pkg).serviceName(),
	}))

}

// DisablePackageService disables gravity service implemented as a gravity package command
func (s *systemdManager) DisablePackageService(pkg loc.Locator) error {
	return s.DisableService(DisableServiceRequest{
		Name: newSystemdUnit(pkg).serviceName(),
	})
}

// IsPackageServiceInstalled checks if the package service is installed
func (s *systemdManager) IsPackageServiceInstalled(pkg loc.Locator) (bool, error) {
	units, err := s.ListPackageServices(DefaultListServiceOptions)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, u := range units {
		if pkg.IsEqualTo(u.Package) {
			return true, nil
		}
	}
	return false, nil
}

// ListPackageServices lists installed package services
func (s *systemdManager) ListPackageServices(opts ListServiceOptions) ([]PackageServiceStatus, error) {
	var services []PackageServiceStatus

	args := []string{"list-units", "--plain", "--no-legend"}
	if opts.All {
		args = append(args, "--all")
	}
	if opts.Type != "" {
		args = append(args, "--type", opts.Type)
	}
	if opts.State != "" {
		args = append(args, "--state", opts.State)
	}
	if opts.Pattern != "" {
		args = append(args, opts.Pattern)
	}
	out, err := invokeSystemctlQuiet(args...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list-units: %v", out)
	}
	for _, line := range strings.Split(out, "\n") {
		words := strings.Fields(line)
		if len(words) < 3 {
			continue
		}
		pkg := parseUnit(words[0])
		if pkg == nil {
			continue
		}
		services = append(services,
			PackageServiceStatus{Package: *pkg, Status: words[2]})
	}
	return services, nil
}

// EnablePackageService enables package service
func (s *systemdManager) EnablePackageService(pkg loc.Locator) error {
	return trace.Wrap(s.EnableService(newSystemdUnit(pkg).serviceName()))
}

// StartPackageService starts package service
func (s *systemdManager) StartPackageService(pkg loc.Locator, noBlock bool) error {
	return trace.Wrap(s.StartService(newSystemdUnit(pkg).serviceName(), noBlock))
}

// StopPackageService stops package servicer
func (s *systemdManager) StopPackageService(pkg loc.Locator) error {
	return trace.Wrap(s.StopService(newSystemdUnit(pkg).serviceName()))
}

// StopPackageServiceCommand returns command that stops package service
func (s *systemdManager) StopPackageServiceCommand(pkg loc.Locator) ([]string, error) {
	path, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []string{path, "stop", newSystemdUnit(pkg).serviceName()}, nil
}

// RestartPackageService restarts package service
func (s *systemdManager) RestartPackageService(pkg loc.Locator) error {
	return s.RestartService(newSystemdUnit(pkg).serviceName())
}

// StatusPackageService returns status of a package service
func (s *systemdManager) StatusPackageService(pkg loc.Locator) (string, error) {
	return s.StatusService(newSystemdUnit(pkg).serviceName())
}

// InstalService installs a new service with the system service manager
func (s *systemdManager) InstallService(req NewServiceRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	template := serviceTemplate{
		Name:        serviceName(req.Name),
		ServiceSpec: req.ServiceSpec,
		Description: fmt.Sprintf("Auto-generated service for the %v", req.Name),
	}
	return trace.Wrap(s.installService(template, req))
}

// InstalMountService installs a new mount service with the system service manager
func (s *systemdManager) InstallMountService(req NewMountServiceRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	template := mountServiceTemplate{
		MountServiceSpec: req.ServiceSpec,
		Name:             req.Name,
		Description:      fmt.Sprintf("Auto-generated mount service for %v", req.Name),
	}
	return trace.Wrap(s.installMountService(template, req.NoBlock))
}

// UninstallService uninstalls service
func (s *systemdManager) UninstallService(req UninstallServiceRequest) error {
	serviceName := serviceName(req.Name)
	logger := log.WithField("service", serviceName)

	serviceStatus, err := s.StatusService(serviceName)
	if err != nil {
		return trace.Wrap(err)
	}

	if serviceStatus == ServiceStatusFailed {
		logger.Warn("Service unit is failed, trying to reset.")
		out, err := invokeSystemctl("reset-failed", serviceName)
		if err != nil {
			logger.WithError(err).Errorf("Failed to reset failed service unit: %v.", out)
		}
	}

	out, err := invokeSystemctl("stop", serviceName)
	if err != nil {
		if IsUnknownServiceError(err) {
			logger.Info("Service not found.")
			return nil
		}
		return trace.Wrap(err, out)
	}

	out, err = invokeSystemctl("disable", serviceName)
	if err != nil {
		return trace.Wrap(err, out)
	}

	// Returns 0 if the unit is in failed state, non-zero otherwise
	// See https://www.freedesktop.org/software/systemd/man/systemctl.html#is-failed%20PATTERN%E2%80%A6
	out, err = invokeSystemctl("is-failed", serviceName)
	status := strings.TrimSpace(out)

	if exitCode := utils.ExitStatusFromError(err); exitCode != nil && *exitCode != 0 {
		unitPath := unitPath(req.Name)
		logger = logger.WithField("path", unitPath)
		if errDelete := os.Remove(unitPath); errDelete != nil {
			if !os.IsNotExist(errDelete) {
				logger.WithError(errDelete).Warn("Failed to remove service unit file.")
			} else {
				logger.Info("Service unit files does not exist.")
			}
		} else {
			logger.Info("Removed service unit file.")
		}
	}

	switch status {
	case ServiceStatusInactive, ServiceStatusUnknown:
		// Ignore the inactive and unknown states
		return nil
	case ServiceStatusFailed:
		return trace.CompareFailed("error stopping service %q: %s", serviceName, out)
	default:
		if err != nil && !IsUnknownServiceError(err) {
			// Results of `systemctl is-failed` are purely informational
			// beyond the state values we already check above
			logger.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"output":        out,
			}).Warn("UninstallService.")
		}
	}

	return nil
}

// DisableService disables service without stopping it
func (s *systemdManager) DisableService(req DisableServiceRequest) error {
	out, err := invokeSystemctl("disable", serviceName(req.Name))
	if err != nil {
		return trace.Wrap(err, "error disabling service %v: %s", req.Name, out)
	}
	return nil
}

// StartService starts service
func (s *systemdManager) StartService(name string, noBlock bool) error {
	var err error
	var out string
	if noBlock {
		out, err = invokeSystemctl("start", name, "--no-block")
	} else {
		out, err = invokeSystemctl("start", name)
	}
	return trace.Wrap(err, "failed to start service %v: %v", name, out)
}

// StopService stops service
func (s *systemdManager) StopService(name string) error {
	out, err := invokeSystemctl("stop", name)
	return trace.Wrap(err, "error stopping %v (%v)", name, out)
}

// RestartService restarts service
func (s *systemdManager) RestartService(name string) error {
	out, err := invokeSystemctl("restart", name)
	return trace.Wrap(err, "failed to restart %v: %v", name, out)
}

// StatusService returns status of a service
func (s *systemdManager) StatusService(name string) (string, error) {
	out, err := invokeSystemctl("is-active", name)
	out = strings.TrimSpace(out)
	// TODO(dmitri): this is a dubious behavior at least w.r.t `unknown` state
	// which one might consider actually unknown. In fact, the `is-active` predicate
	// _always_ returns a state for a (arbitrary, even non-existent) service, and a
	// non-zero exit code if the status is not 'active'
	//
	// do not report error in case if status is known
	switch out {
	case ServiceStatusActive, ServiceStatusInactive,
		ServiceStatusFailed, ServiceStatusUnknown,
		ServiceStatusActivating, ServiceStatusDeactivating:
		return out, nil
	}
	return out, err
}

// EnableService enables service
func (s *systemdManager) EnableService(name string) error {
	out, err := invokeSystemctl("enable", name)
	return trace.Wrap(err, "failed to enable %v: %v", name, out)
}

// Version returns systemd version
func (s *systemdManager) Version() (int, error) {
	out, err := invokeSystemctl("--version")
	if err != nil {
		return 0, trace.Wrap(err, "failed to get systemd version: %s", out)
	}
	version, err := utils.ParseSystemdVersion(out)
	if err != nil {
		return 0, trace.Wrap(err, "failed to parse systemd version output: %s", out)
	}
	return version, nil
}

// supportsTasksAccounting returns true if systemd supports tasks accounting on the machine,
// in case of an error it falls back to "false" and the error gets logged
func (s *systemdManager) supportsTasksAccounting() bool {
	version, err := s.Version()
	if err != nil {
		log.Errorf("failed to determine systemd version: %v", trace.DebugReport(err))
		return false
	}
	return version >= defaults.SystemdTasksMinVersion
}

func invokeSystemctlQuiet(args ...string) (string, error) {
	out, err := exec.Command("systemctl", append(args, "--no-pager")...).CombinedOutput()
	return string(out), trace.Wrap(err)
}

func invokeSystemctl(args ...string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("systemctl", append(args, "--no-pager")...)
	err := utils.ExecL(cmd, &out, log)
	return out.String(), trace.Wrap(err)
}

// unitPath returns the default path for the systemd unit with the given name.
// If the name is an absolute path, it is returned verbatim
func unitPath(name string) (path string) {
	if filepath.IsAbs(name) {
		return name
	}
	return DefaultUnitPath(name)
}

// PackageServiceName returns the name of the package service
// for the specified package locator
func PackageServiceName(loc loc.Locator) string {
	return newSystemdUnit(loc).serviceName()
}

// DefaultUnitPath returns the default path for the specified systemd unit
func DefaultUnitPath(name string) (path string) {
	return filepath.Join(defaults.SystemUnitDir, SystemdNameEscape(name))
}

// DefaultListServiceOptions specifies the default configuration to list package services
var DefaultListServiceOptions = ListServiceOptions{
	All:  true,
	Type: UnitTypeService,
}

// serviceName returns just the name portion of the unit path.
// If the name is already a relative service name, it is returned verbatim
func serviceName(name string) string {
	return filepath.Base(name)
}
