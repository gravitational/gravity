/*
Copyright 2020 Gravitational, Inc.

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

// package auditlog implements support for manipulating kernel audit system
package auditlog

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gravitational/gravity/lib/log"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// New returns a new instance of the logger
func New() *Auditlog {
	return &Auditlog{
		log: log.New(logrus.WithField(trace.Component, "auditlog")),
	}
}

// AddDefaultRules adds default audit rules for all known domains
func (r *Auditlog) AddDefaultRules() error {
	subjtypeArg := func(subjtype string) string {
		return fmt.Sprintf("subj_type=%v", subjtype)
	}
	syscallArg := strings.Join(syscalls, ",")
	rule := []string{
		"-a", "always,exit", "-F", "success=0", "-F", "arch=b64", "-S", syscallArg, "-k", auditKey,
	}
	for _, domain := range Domains {
		rule = append(rule, "-F", subjtypeArg(domain))
	}
	cmd := exec.Command(auditctlBin, rule...)
	logger := r.log.WithField("cmd", cmd.Args)
	w := logger.Writer()
	defer w.Close()
	cmd.Stdout = w
	cmd.Stderr = w
	logger.Info("Set up audit rules.")
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "failed to set up audit rule for process")
	}
	return nil
}

// RemoveRules removes previously configured audit rules
func (r *Auditlog) RemoveRules() error {
	cmd := exec.Command(auditctlBin, "-D", "-k", auditKey)
	r.log.WithField("cmd", cmd.Args).Info("Remove audit rules.")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, "failed to remove audit rules: %s", out)
	}
	return nil
}

// Auditlog manages audit rules on the host
type Auditlog struct {
	log log.Logger
}

// Domains lists all gravity SELinux process domains for auditing
var Domains = []string{
	// gravity_domain
	"gravity_t",
	// gravity_container_domain
	"gravity_container_runtime_t",
	"gravity_container_init_t",
	"gravity_container_systemctl_t",
	"gravity_kubernetes_t",
	"gravity_service_t",
	"gravity_container_t",
	"gravity_container_system_t",
	"gravity_container_logger_t",
}

var syscalls = []string{
	"open", "creat", "rename", "unlink", "mkdir", "rmdir", "chown", "chmod", "symlink", "read",
	"openat", "truncate", "renameat", "unlinkat", "mkdirat",
	"execve", "setxattr",
	"connect", "bind", "accept", "sendto", "recvfrom",
}

const (
	auditKey    = "gravity"
	auditctlBin = "/sbin/auditctl"
)
