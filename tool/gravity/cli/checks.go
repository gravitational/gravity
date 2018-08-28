package cli

import (
	"fmt"
	"io/ioutil"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/schema"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

func checkManifest(env *localenv.LocalEnvironment, manifestPath, profileName string, autoFix bool) error {
	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return trace.Wrap(err)
	}

	manifest, err := schema.ParseManifestYAML(data)
	if err != nil {
		return trace.Wrap(err)
	}

	result, err := checks.ValidateLocal(checks.LocalChecksRequest{
		Manifest: *manifest,
		Role:     profileName,
		AutoFix:  autoFix,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var failedErr, fixableErr error
	if len(result.Failed) > 0 {
		failedErr = trace.BadParameter(fmt.Sprintf("The following checks failed:\n%v",
			checks.FormatFailedChecks(result.Failed)))
	}
	if len(result.Fixable) > 0 {
		fixableErr = trace.BadParameter(fmt.Sprintf("The following checks failed, provide --autofix flag to let gravity to autofix them:\n%v",
			checks.FormatFailedChecks(result.Fixable)))
	}

	return trace.NewAggregate(failedErr, fixableErr)
}

func printFailedChecks(failed []*pb.Probe) {
	if len(failed) == 0 {
		return
	}

	fmt.Printf("Failed checks:\n")
	fmt.Printf(checks.FormatFailedChecks(failed))
}
