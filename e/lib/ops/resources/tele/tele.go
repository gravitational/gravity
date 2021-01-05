package tele

import (
	"strings"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

func init() {
	modules.SetResources(&teleResources{})
}

// Resources is controller for resources managed by "tele resource" subcommands
type Resources struct {
	// Config is the controller configuration
	Config
}

// Config is tele resource controller configuration
type Config struct {
	// Operator is Ops Center client
	Operator *opsclient.Client
	// Apps is Ops Center apps service client
	Apps app.Applications
	// Silent provides methods for printing
	localenv.Silent
}

// New creates a new tele resource controller
func New(config Config) *Resources {
	return &Resources{
		Config: config,
	}
}

// Create creates the provided resource
func (r *Resources) Create(req resources.CreateRequest) error {
	switch req.Resource.Kind {
	case storage.KindCluster:
		cluster, err := storage.UnmarshalCluster(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := cluster.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		operationKey, err := ops.CreateCluster(r.Operator, cluster)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Initated operation %v to create cluster %q\n",
			operationKey.OperationID, cluster.GetName())
	case "":
		return trace.BadParameter("missing resource kind")
	default:
		return trace.BadParameter("unsupported resource %q, supported are: %v",
			req.Resource.Kind, SupportedResources)
	}
	return nil
}

// GetCollection retrieves a collection of specified resources
func (r *Resources) GetCollection(req resources.ListRequest) (resources.Collection, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	switch req.Kind {
	case storage.KindCluster:
		clusters, err := ops.GetClusters(r.Operator, req.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &clusterCollection{clusters: clusters}, nil
	case storage.KindApp:
		apps, err := r.Apps.ListApps(app.ListAppsRequest{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return appCollection(apps), nil
	}
	return nil, trace.BadParameter("unsupported resource %q, supported are: %v",
		req.Kind, SupportedResources)
}

// Remove removes the specified resource
func (r *Resources) Remove(req resources.RemoveRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	switch req.Kind {
	case storage.KindApp:
		locator, err := loc.MakeLocator(req.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		err = r.Apps.DeleteApp(app.DeleteRequest{
			Package: *locator,
		})
		if err != nil {
			if !trace.IsNotFound(err) || !req.Force {
				return trace.Wrap(err)
			}
		}
		r.Printf("Application %v removed\n", req.Name)
	case storage.KindCluster:
		operation, err := ops.RemoveCluster(r.Operator, req.Name)
		if err != nil {
			if trace.IsNotFound(err) && req.Force {
				r.Printf("Cluster %v is not found\n", req.Name)
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Initiated operation %v to uninstall cluster %q\n",
			operation.OperationID, req.Name)
	case "":
		return trace.BadParameter("missing resource kind")
	default:
		return trace.BadParameter("unsupported resource %q, supported are: %v",
			req.Kind, SupportedResources)
	}
	return nil
}

// SupportedResources returns a list of resources that can be created/viewed
func (*teleResources) SupportedResources() []string {
	return SupportedResources
}

// SupportedResourcesToRemove returns a list of resources that can be removed
func (*teleResources) SupportedResourcesToRemove() []string {
	return SupportedResources
}

// CanonicalKind translates the specified kind to canonical form.
// Returns an empty string if no canonical form exists
func (*teleResources) CanonicalKind(kind string) string {
	return canonicalKind(kind)
}

// canonicalKind translates the specified kind to canonical form.
// Returns an empty string if no canonical form exists
func canonicalKind(kind string) string {
	switch strings.ToLower(kind) {
	case storage.KindApp, "apps":
		return storage.KindApp
	case storage.KindCluster, "clusters":
		return storage.KindCluster
	default:
		return kind
	}
}

type teleResources struct{}

// SupportedResources is a list of resources supported by
// "tele resource create/get/rm" subcommands
var SupportedResources = []string{
	storage.KindCluster,
	storage.KindApp,
}
