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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-systemd/unit"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

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
ExecStart={{.StartCommand}}
{{if .StartPreCommand}}ExecStartPre={{.StartPreCommand}}{{end}}
{{if .StartPostCommand}}ExecStartPost={{.StartPostCommand}}{{end}}
{{if .StopCommand}}ExecStop={{.StopCommand}}{{end}}
{{if .StopPostCommand}}ExecStopPost={{.StopPostCommand}}{{end}}
{{if .LimitNoFile}}LimitNOFILE={{.LimitNoFile}}{{else}}LimitNOFILE=100000{{end}}
{{if .KillMode}}KillMode={{.KillMode}}{{end}}
{{if .KillSignal}}KillSignal={{.KillSignal}}{{end}}
{{if .Restart}}Restart={{.Restart}}{{end}}
{{if .TimeoutStopSec}}TimeoutStopSec={{.TimeoutStopSec}}{{end}}
{{if .RestartSec}}RestartSec={{.RestartSec}}{{end}}
{{if .RemainAfterExit}}RemainAfterExit=yes{{end}}
{{range $k, $v := .Environment}}Environment={{$k}}={{$v}}
{{end}}
{{if .TasksMax}}TasksMax={{.TasksMax}}{{end}}

{{if .WantedBy}}
[Install]
WantedBy={{.WantedBy}}
{{end}}
`

	// Should we make the target configurable too?
	systemdUnitFileDir = "/etc/systemd/system/"

	systemdUnitFileSuffix = ".service"
	systemdMountSuffix    = ".mount"

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
	unit = strings.TrimSuffix(unit, systemdUnitFileSuffix)
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

// parseMount parses mount point from the mount service name.
//
// Systemd mount unit name consists of all mount point path elements
// separated by dashes so mount service with name "var-lib-gravity.mount"
// has a mount point /var/lib/gravity.
func parseMount(mount string) string {
	name := strings.TrimSuffix(mount, systemdMountSuffix)
	return unit.UnitNamePathUnescape(name)
}

func (u *systemdUnit) serviceName() string {
	return strings.Join([]string{
		servicePrefix, u.pkg.Repository, u.pkg.Name, u.pkg.Version},
		systemdServiceDelimiter) + systemdUnitFileSuffix
}

func (u *systemdUnit) servicePath() string {
	return filepath.Join(systemdUnitFileDir, u.serviceName())
}

func (s *systemdManager) installService(service serviceTemplate, noBlock, noStart bool) error {
	service.Environment = map[string]string{
		defaults.PathEnv: defaults.PathEnvVal,
	}

	servicePath := filepath.Join(systemdUnitFileDir, SystemdNameEscape(service.Name))
	f, err := os.Create(servicePath)
	if err != nil {
		return trace.Wrap(err,
			"error creating systemd unit file at %v", servicePath)
	}
	defer f.Close()

	err = serviceUnitTemplate.Execute(f, service)
	if err != nil {
		return trace.Wrap(err, "error rendering template")
	}

	if err := s.EnableService(service.Name); err != nil {
		return trace.Wrap(err, "error enabling the service")
	}

	if noStart {
		log.Infof("Not starting service %v.", service.Name)
		return nil
	}

	if err := s.StartService(service.Name, noBlock); err != nil {
		return trace.Wrap(err, "error starting the service")
	}

	return nil
}

func (s *systemdManager) installMountService(service mountServiceTemplate, noBlock bool) error {
	servicePath := filepath.Join(systemdUnitFileDir, SystemdNameEscape(service.Name))
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

	return trace.Wrap(s.installService(template, req.NoBlock, req.NoStart))
}

// UninstallPackageService uninstalls gravity service implemented as a gravity package command
func (s *systemdManager) UninstallPackageService(pkg loc.Locator) error {
	return trace.Wrap(s.UninstallService(newSystemdUnit(pkg).serviceName()))

}

// DisablePackageService disables gravity service implemented as a gravity package command
func (s *systemdManager) DisablePackageService(pkg loc.Locator) error {
	return s.DisableService(newSystemdUnit(pkg).serviceName())
}

// IsPackageServiceInstalled checks if the package service is installed
func (s *systemdManager) IsPackageServiceInstalled(pkg loc.Locator) (bool, error) {
	units, err := s.ListPackageServices()
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
func (s *systemdManager) ListPackageServices() ([]PackageServiceStatus, error) {
	var services []PackageServiceStatus

	out, err := invokeSystemctl("list-units", "--plain", "--no-legend")
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
		services = append(
			services, PackageServiceStatus{Package: *pkg, Status: words[2]})
	}
	return services, nil
}

// ListMounts lists all mount services.
func (s *systemdManager) ListMounts() (mounts []MountStatus, err error) {
	out, err := invokeSystemctl("list-units", "--plain", "--no-legend")
	if err != nil {
		return nil, trace.Wrap(err, "failed to list-units: %s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		words := strings.Fields(line)
		if len(words) < 3 || !strings.HasSuffix(words[0], systemdMountSuffix) {
			continue
		}
		mounts = append(mounts, MountStatus{
			Name:       words[0],
			MountPoint: parseMount(words[0]),
			Status:     words[2],
		})
	}
	return mounts, nil
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
		Name:        req.Name,
		ServiceSpec: req.ServiceSpec,
		Description: fmt.Sprintf("Auto-generated service for the %v", req.Name),
	}
	return trace.Wrap(s.installService(template, req.NoBlock, req.NoStart))
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
func (s *systemdManager) UninstallService(name string) error {
	out, err := invokeSystemctl("disable", name)
	if err != nil {
		return trace.Wrap(err, "error disabling service %v: %v", name, out)
	}

	out, err = invokeSystemctl("stop", name)
	if err != nil {
		return trace.Wrap(err, "error stopping service %v: %s", name, out)
	}

	out, err = invokeSystemctl("is-failed", name)
	status := strings.TrimSpace(out)

	switch status {
	case ServiceStatusInactive:
		// Ignore the inactive state
		return nil
	case ServiceStatusFailed:
		return trace.CompareFailed("error stopping service %v: %s", name, out)
	default:
		if err != nil {
			// Results of `systemctl is-failed` are purely informational
			// beyond the state values we already check above
			log.Warnf("service %v status: %s", name, out)
			log.Debug(trace.DebugReport(err))
		}
	}

	return nil
}

// DisableService disables service without stopping it
func (s *systemdManager) DisableService(name string) error {
	out, err := invokeSystemctl("disable", name)
	if err != nil {
		return trace.Wrap(err, "error disabling service %v: %v", name, out)
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
	// do not report error in case if status is known
	switch out {
	case ServiceStatusActive, ServiceStatusFailed, ServiceStatusActivating,
		ServiceStatusUnknown, ServiceStatusInactive:
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

func invokeSystemctl(args ...string) (string, error) {
	cmd := exec.Command("systemctl", append(args, "--no-pager")...)
	out := &bytes.Buffer{}
	err := utils.ExecL(cmd, out, log.WithField(trace.Component, constants.ComponentSystem))
	return out.String(), trace.Wrap(err)
}
