package cli

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/report"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/dustin/go-humanize"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	yaml "gopkg.in/yaml.v2"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

// systemReport collects system diagnostics and outputs them as a (optionally compressed) tarball
// to the stdout.
// filters define the specific diagnostics to collect ('system', 'kubernetes'),
// if empty all diagnostics are collected.
func systemReport(env *localenv.LocalEnvironment, filters []string, compressed bool) error {
	runner := utils.NewRunner(nil)

	var collectors report.Collectors
	for _, filter := range teleutils.Deduplicate(filters) {
		switch filter {
		case constants.ReportFilterSystem:
			collectors = append(collectors, report.SystemInfo()...)
			collectors = append(collectors, packageCollector{env})
		case constants.ReportFilterKubernetes:
			collectors = append(collectors, report.KubernetesInfo(runner)...)
		}
	}
	if len(filters) == 0 {
		collectors = append(collectors, report.SystemInfo()...)
		collectors = append(collectors, report.KubernetesInfo(runner)...)
		collectors = append(collectors, packageCollector{env})
	}

	dir, err := ioutil.TempDir("", "report")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer os.RemoveAll(dir)

	rw := report.NewFileWriter(dir)
	err = collectors.Collect(rw, runner)
	if err != nil {
		log.Errorf("failed to collect some diagnostics: %v", trace.DebugReport(err))
	}

	reader, writer := io.Pipe()

	go func() {
		var output io.WriteCloser = writer
		if compressed {
			output = gzip.NewWriter(writer)
		}
		err := archive.CompressDirectory(dir, output)
		if compressed {
			output.Close()
		}
		writer.CloseWithError(err)
	}()

	_, err = io.Copy(os.Stdout, reader)

	return trace.Wrap(err)
}

// Collect iterates through the system packages and outputs them
// using the specified reportWriter.
func (r packageCollector) Collect(reportWriter report.Writer, runner utils.CommandRunner) error {
	w, err := reportWriter("gravity-packages.yaml")
	if err != nil {
		return trace.Wrap(err)
	}
	framer := serializer.YAMLFramer.NewFrameWriter(w)
	defer w.Close()

	return foreachPackage(r.env, "", defaults.GravityServiceURL, formatPackage(framer))
}

func formatPackage(w io.Writer) func(pack.PackageEnvelope) error {
	return func(env pack.PackageEnvelope) error {
		pkg := &envelope{
			Name:      locator(env.Locator),
			Size:      size(env.SizeBytes),
			Digest:    env.SHA512,
			Labels:    env.RuntimeLabels,
			Hidden:    env.Hidden,
			Encrypted: env.Encrypted,
			Type:      env.Type,
			Created:   env.Created,
		}
		if !env.Encrypted {
			pkg.Manifest = manifest(env.Manifest)
		}
		out, err := yaml.Marshal(pkg)
		if err != nil {
			return trace.Wrap(err)
		}

		_, err = w.Write(out)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
}

type packageCollector struct {
	env *localenv.LocalEnvironment
}

type envelope struct {
	Name      locator           `yaml:"name"`
	Size      size              `yaml:"size"`
	Digest    string            `yaml:"digest"`
	Labels    map[string]string `yaml:"labels,omitempty"`
	Hidden    bool              `yaml:"hidden"`
	Encrypted bool              `yaml:"encrypted"`
	Type      string            `yaml:"type,omitempty"`
	Created   time.Time         `yaml:"created"`
	Manifest  manifest          `yaml:"manifest,omitempty"`
}

// MarshalYAML formats this locator value as a package name in YAML
func (r locator) MarshalYAML() (interface{}, error) {
	return (loc.Locator)(r).String(), nil
}

type locator loc.Locator

// MarshalYAML formats this size as human-readable value in YAML
func (r size) MarshalYAML() (interface{}, error) {
	return humanize.Bytes(uint64(r)), nil
}

type size int64

// MarshalYAML formats this manifest in YAML as text
func (r manifest) MarshalYAML() (interface{}, error) {
	return string(r), nil
}

type manifest []byte
