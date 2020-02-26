package selinux

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	liblog "github.com/gravitational/gravity/lib/log"
	"github.com/gravitational/gravity/lib/system/selinux/internal/policy"
	"github.com/gravitational/gravity/lib/system/selinux/internal/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ApplyFileContests restores the file contexts in specified list of paths
func ApplyFileContexts(ctx context.Context, out io.Writer, paths ...string) error {
	args := []string{"-Rvi"}
	args = append(args, paths...)
	cmd := exec.CommandContext(ctx, "restorecon", args...)
	cmd.Stdout = out
	cmd.Stderr = out
	return trace.Wrap(cmd.Run())
}

// ShouldLabelVolume determines if the specified label is valid
func ShouldLabelVolume(label string) bool {
	return label != ""
}

func renderFcontext(w io.Writer, stateDir string, fcontextTemplate io.Reader, renderer commandRenderer) error {
	b, err := ioutil.ReadAll(fcontextTemplate)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	tmpl, err := template.New("fcontext").Parse(string(b))
	if err != nil {
		return trace.Wrap(err)
	}
	var buf bytes.Buffer
	var values = struct {
		StateDir string
	}{
		StateDir: stateDir,
	}
	if err := tmpl.Execute(&buf, values); err != nil {
		return trace.Wrap(err)
	}
	items, err := schema.ParseFcontextFile(&buf)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, item := range items {
		if _, err := fmt.Fprint(w, renderer(item), "\n"); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func withPolicy(path string) (io.ReadCloser, error) {
	f, err := policy.Policy.Open(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

type policyFileReader interface {
	OpenFile(name string) (io.ReadCloser, error)
}

func (r policyFileReaderFunc) OpenFile(name string) (io.ReadCloser, error) {
	return r(name)
}

type policyFileReaderFunc func(name string) (io.ReadCloser, error)

type commandRenderer func(schema.FcontextFileItem) string

func installPolicyFile(ctx context.Context, path string, r io.Reader) error {
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	logger.WithField("path", path).Info("Install policy file.")
	if err := utils.CopyReaderWithPerms(path, r, defaults.SharedReadMask); err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.CommandContext(ctx, "semodule", "--install", path)
	w := logger.Writer()
	defer w.Close()
	cmd.Stdout = w
	cmd.Stderr = w
	return trace.Wrap(cmd.Run())
}

func importLocalChangesFromReader(ctx context.Context, r io.Reader) error {
	cmd := exec.CommandContext(ctx, "semanage", "import")
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	w := logger.Writer()
	defer w.Close()
	cmd.Stdin = r
	cmd.Stdout = w
	cmd.Stderr = w
	return trace.Wrap(cmd.Run())
}

func removePolicy() error {
	// Leave the container policy module in-place as we might not be
	// the only client
	return removePolicyByName("gravity")
}

func removePolicyByName(module string) error {
	logger := liblog.New(log.WithField(trace.Component, "selinux"))
	logger.WithField("module", module).Info("Remove policy module.")
	cmd := exec.Command("semodule", "--remove", module)
	w := logger.Writer()
	defer w.Close()
	cmd.Stdout = w
	cmd.Stderr = w
	return trace.Wrap(cmd.Run())
}
