package hooks

import (
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/rigging"
	"gopkg.in/check.v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigureSuite struct{}

var _ = check.Suite(&ConfigureSuite{})

func (s *ConfigureSuite) TestConfigureMetadata(c *check.C) {
	job := &batchv1.Job{}

	nodeSelector := map[string]string{"role": "master"}
	deadline := time.Duration(10 * time.Second)
	err := configureMetadata(job, Params{
		NodeSelector: nodeSelector,
		JobDeadline:  deadline,
	})
	c.Assert(err, check.IsNil)

	c.Assert(job.TypeMeta, check.DeepEquals, metav1.TypeMeta{Kind: rigging.KindJob, APIVersion: rigging.BatchAPIVersion})
	c.Assert(job.ObjectMeta.Namespace, check.Equals, defaults.KubeSystemNamespace)
	c.Assert(job.Spec.Template.Spec.NodeSelector, check.DeepEquals, nodeSelector)
	c.Assert(*job.Spec.ActiveDeadlineSeconds, check.Equals, int64(deadline.Seconds()))
	c.Assert(job.Spec.Template.Spec.SecurityContext, check.DeepEquals, defaults.HookSecurityContext())
}
