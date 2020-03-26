package auditlog

import (
	"fmt"
	"os/exec"

	"github.com/gravitational/gravity/lib/log"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// New returns a new instance of the logger
func New(pid int) *Auditlog {
	return &Auditlog{
		log: log.New(logrus.WithFields(logrus.Fields{
			trace.Component: "auditlog",
			"pid":           pid,
		})),
		pid: pid,
	}
}

// AddDefaultRules adds default audit rules for the underlying process
func (r *Auditlog) AddDefaultRules() error {
	pidArg := fmt.Sprintf("pid=%d", r.pid)
	// exeArg := fmt.Sprintf("exe=%v", utils.Exe.Path)
	subjtypeArg := func(subjtype string) string {
		return fmt.Sprintf("subj_type=%v", subjtype)
	}
	rules := [][]string{
		// Current process
		{"-a", "always,exit", "-F", "success=0", "-F", pidArg, "-F", "arch=b64", "-S", "all", "-k", auditKey},
	}
	// TODO: split gravity vs the rest and add another rule without binary for the rest (or lose the exe)
	rule := []string{"-a", "always,exit", "-F", "success=0", "-F", "arch=b64", "-S", "all", "-k", auditKey}
	for _, domain := range Domains {
		rule = append(rule, "-F", subjtypeArg(domain))
	}
	rules = append(rules, rule)
	for _, rule := range rules {
		cmd := exec.Command(auditctlBin, rule...)
		logger := r.log.WithField("cmd", cmd.Args)
		w := logger.Writer()
		defer w.Close()
		cmd.Stdout = w
		cmd.Stderr = w
		logger.Info("Set up audit rule.")
		if err := cmd.Run(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err,
				// "output": string(out),
			}).Warn("Failed to set up audit rule.")
			// return trace.Wrap(err, "failed to set up audit rule for process: %s", out)
			return trace.Wrap(err, "failed to set up audit rule for process")
		}
	}
	return nil
}

// RemoveRules removes audit rules for the underlying process
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
	pid int
}

// Domains lists all gravity SELinux process Domains for auditing
var Domains = []string{
	// gravity_domain
	"gravity_t",
	"gravity_installer_t",
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

const (
	auditKey    = "gravity"
	auditctlBin = "/sbin/auditctl"
)
