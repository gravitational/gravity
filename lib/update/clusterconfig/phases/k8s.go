package phases

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func removeService(ctx context.Context, name string, opts *metav1.DeleteOptions, services corev1.ServiceInterface) error {
	// FIXME: timeout
	b := utils.NewExponentialBackOff(1 * time.Minute)
	return utils.RetryTransient(ctx, b, func() error {
		err := services.Delete(name, opts)
		if err != nil {
			return rigging.ConvertError(err)
		}
		return nil
	})
}
