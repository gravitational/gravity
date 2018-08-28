package utils

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

// CheckHelm checks that helm binary is properly installed
func CheckHelm() error {
	_, err := exec.LookPath("helm")
	if err != nil {
		return trace.BadParameter(
			"helm binary is not found or not executable (%q), check https://docs.helm.sh/using_helm/#installing-helm for details", err)
	}
	buf := &bytes.Buffer{}
	err = Exec(exec.Command("helm", "plugin", "list"), buf)
	if err != nil {
		return trace.BadParameter("failed to run 'helm plugin list' command: %v", err)
	}

	if !strings.Contains(buf.String(), "template") {
		return trace.BadParameter("helm template plugin is not found in installed plugins, install using 'helm plugin install https://github.com/technosophos/helm-template'")
	}
	return nil
}
