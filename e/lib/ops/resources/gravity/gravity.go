package gravity

import (
	"context"

	"github.com/gravitational/gravity/e/lib/modules"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	libkube "github.com/gravitational/gravity/lib/kubernetes"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Resources extends OSS gravity resources controller
type Resources struct {
	// Config is the controller configuration
	Config
	// cluster is the local cluster
	cluster *ossops.Site
}

// Config is gravity resource controller configuration
type Config struct {
	// Resources is the OSS resources controller
	*gravity.Resources
	// Operator is the operator service
	Operator ops.Operator
}

// New creates a new gravity resource controller
func New(config Config) (*Resources, error) {
	localCluster, err := config.Operator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Resources{
		Config:  config,
		cluster: localCluster,
	}, nil
}

// Create creates the provided resource
func (r *Resources) Create(ctx context.Context, req resources.CreateRequest) error {
	r.Log.Infof("%s.", req)
	kind := modules.CanonicalKind(req.Resource.Kind)
	switch kind {
	case teleservices.KindRole:
		role, err := teleservices.GetRoleMarshaler().UnmarshalRole(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertRole(ctx, req.SiteKey, role); err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Created role %q\n", role.GetName())
	case teleservices.KindOIDCConnector:
		conn, err := teleservices.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertOIDCConnector(ctx, req.SiteKey, conn); err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Created OIDC connector %q\n", conn.GetName())
	case teleservices.KindSAMLConnector:
		conn, err := teleservices.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertSAMLConnector(ctx, req.SiteKey, conn); err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Created SAML connector %q\n", conn.GetName())
	case teleservices.KindTrustedCluster:
		cluster, err := storage.UnmarshalTrustedCluster(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpsertTrustedCluster(ctx, req.SiteKey, cluster)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Created trusted cluster %q\n", cluster.GetName())
	case storage.KindEndpoints:
		// TODO(r0mant): Endpoints resource currently can only be
		//               updated on a local cluster because it will
		//               need to restart gravity-site pods.
		if !r.cluster.Key().IsEqualTo(req.SiteKey) {
			return trace.BadParameter("cannot update Gravity Hub endpoints on a remote cluster")
		}
		endpoints, err := storage.UnmarshalEndpoints(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpdateClusterEndpoints(ctx, r.cluster.Key(), endpoints)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Println("Updated cluster endpoints, restarting cluster controller pods")
	default:
		// not enterprise-specific resource, use OSS controller
		return r.Resources.Create(ctx, req)
	}
	switch req.Resource.Kind {
	case storage.KindEndpoints:
		client, _, err := httplib.GetClusterKubeClient(r.cluster.DNSConfig.Addr())
		if err != nil {
			return trace.Wrap(err, "failed to create Kubernetes client")
		}
		err = deleteGravityPods(client)
		if err != nil {
			return trace.Wrap(err, "failed to restart gravity-site pods, "+
				"please restart them manually for the changes to take effect:\n"+
				"$ kubectl delete pods -n kube-system -l app=gravity-site")
		}
	}
	return nil
}

// GetCollection retrieves a collection of specified resources
func (r *Resources) GetCollection(req resources.ListRequest) (resources.Collection, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	r.Log.Debugf("%s.", req)
	kind := modules.CanonicalKind(req.Kind)
	switch kind {
	case teleservices.KindRole:
		if req.Name != "" {
			role, err := r.Operator.GetRole(req.SiteKey, req.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &roleCollection{roles: []teleservices.Role{role}}, nil
		}
		roles, err := r.Operator.GetRoles(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &roleCollection{roles: roles}, nil
	case teleservices.KindOIDCConnector:
		if req.Name != "" {
			connector, err := r.Operator.GetOIDCConnector(req.SiteKey, req.Name, req.WithSecrets)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &oidcCollection{connectors: []teleservices.OIDCConnector{connector}}, nil
		}
		connectors, err := r.Operator.GetOIDCConnectors(req.SiteKey, req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oidcCollection{connectors: connectors}, nil
	case teleservices.KindSAMLConnector:
		if req.Name != "" {
			connector, err := r.Operator.GetSAMLConnector(req.SiteKey, req.Name, req.WithSecrets)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &samlCollection{connectors: []teleservices.SAMLConnector{connector}}, nil
		}
		connectors, err := r.Operator.GetSAMLConnectors(req.SiteKey, req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &samlCollection{connectors: connectors}, nil
	case teleservices.KindAuthConnector: // special case: returns connectors of all kinds
		oidc, err := r.Operator.GetOIDCConnectors(req.SiteKey, req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		github, err := r.Operator.GetGithubConnectors(req.SiteKey, req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		saml, err := r.Operator.GetSAMLConnectors(req.SiteKey, req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors := make([]teleservices.Resource, 0, len(oidc)+len(github)+len(saml))
		for _, c := range oidc {
			connectors = append(connectors, c)
		}
		for _, c := range github {
			connectors = append(connectors, c)
		}
		for _, c := range saml {
			connectors = append(connectors, c)
		}
		return &authConnectorCollection{connectors: connectors}, nil
	case teleservices.KindTrustedCluster:
		if req.Name != "" {
			cluster, err := r.Operator.GetTrustedCluster(req.SiteKey, req.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &trustedClusterCollection{
				clusters: []storage.TrustedCluster{cluster},
			}, nil
		}
		clusters, err := r.Operator.GetTrustedClusters(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &trustedClusterCollection{clusters: clusters}, nil
	case storage.KindEndpoints:
		endpoints, err := r.Operator.GetClusterEndpoints(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &endpointsCollection{endpoints: endpoints}, nil
	}
	// not enterprise-specific resource, use OSS controller
	return r.Resources.GetCollection(req)
}

// Remove removes the specified resource
func (r *Resources) Remove(ctx context.Context, req resources.RemoveRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	r.Log.Infof("%s.", req)
	kind := modules.CanonicalKind(req.Kind)
	switch kind {
	case teleservices.KindRole:
		if err := r.Operator.DeleteRole(ctx, req.SiteKey, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Role %q has been deleted\n", req.Name)
	case teleservices.KindOIDCConnector:
		if err := r.Operator.DeleteOIDCConnector(ctx, req.SiteKey, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("OIDC connector %q has been deleted\n", req.Name)
	case teleservices.KindSAMLConnector:
		if err := r.Operator.DeleteSAMLConnector(ctx, req.SiteKey, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("SAML connector %q has been deleted\n", req.Name)
	case teleservices.KindTrustedCluster:
		err := r.Operator.DeleteTrustedCluster(ctx,
			ops.DeleteTrustedClusterRequest{
				AccountID:          req.AccountID,
				ClusterName:        req.SiteDomain,
				TrustedClusterName: req.Name,
			})
		if err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Trusted cluster %q has been deleted\n", req.Name)
	default:
		// not enterprise-specific resource, use OSS controller
		return r.Resources.Remove(ctx, req)
	}
	return nil
}

// Validate checks whether the specified resource
// represents a valid resource.
func Validate(resource storage.UnknownResource) (err error) {
	kind := modules.CanonicalKind(resource.Kind)
	switch kind {
	case teleservices.KindRole:
		_, err = teleservices.GetRoleMarshaler().UnmarshalRole(resource.Raw)
	case teleservices.KindOIDCConnector:
		_, err = teleservices.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(resource.Raw)
	case teleservices.KindSAMLConnector:
		_, err = teleservices.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(resource.Raw)
	case teleservices.KindTrustedCluster:
		_, err = storage.UnmarshalTrustedCluster(resource.Raw)
	case storage.KindEndpoints:
		_, err = storage.UnmarshalEndpoints(resource.Raw)
	default:
		// not enterprise-specific resource, use OSS controller
		err = gravity.Validate(resource)
	}
	return trace.Wrap(err)
}

func deleteGravityPods(client *kubernetes.Clientset) error {
	err := libkube.DeletePods(client, metav1.NamespaceSystem,
		defaults.GravitySiteSelector)
	return trace.Wrap(err)
}
