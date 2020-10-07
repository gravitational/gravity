/*
Copyright 2018 Gravitational, Inc.

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

package gravity

import (
	"context"

	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/validate"

	"github.com/fatih/color"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Resources is a controller that manages cluster local resources
type Resources struct {
	// Config is the controller configuration
	Config
	// Log is used for logging.
	Log logrus.FieldLogger
}

// Config is gravity resource controller configuration
type Config struct {
	// Operator is local cluster ops client
	Operator ops.Operator
	// CurrentUser is the currently logged in user
	CurrentUser string
	// Silent provides methods for printing
	localenv.Silent
	// ClusterOperationHandler specifies the optional handler
	// for resources that require special handling
	ClusterOperationHandler
}

// Check makes sure the config is valid
func (c Config) Check() error {
	if c.Operator == nil {
		return trace.BadParameter("missing Operator")
	}
	return nil
}

// New creates a new gravity resource controller
func New(config Config) (*Resources, error) {
	err := config.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Resources{
		Config: config,
		Log:    logrus.WithField(trace.Component, "resources"),
	}, nil
}

// Create creates the provided resource
func (r *Resources) Create(ctx context.Context, req resources.CreateRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	r.Log.Infof("%s.", req)
	switch req.Resource.Kind {
	case teleservices.KindGithubConnector:
		conn, err := teleservices.GetGithubConnectorMarshaler().Unmarshal(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertGithubConnector(ctx, req.SiteKey, conn); err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Created Github connector %q\n", conn.GetName())
	case teleservices.KindUser:
		user, err := teleservices.GetUserMarshaler().UnmarshalUser(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertUser(ctx, req.SiteKey, user); err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Created user %q\n", user.GetName())
	case storage.KindToken:
		token, err := storage.GetTokenMarshaler().UnmarshalToken(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if req.Owner != "" {
			token.SetUser(req.Owner)
		} else if token.GetUser() == "" {
			token.SetUser(r.CurrentUser)
		}
		if err := token.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		// using existing keys API here which is compatible so we don't
		// have to roll out separate tokens API for now
		_, err = r.Operator.CreateAPIKey(ctx, ops.NewAPIKeyRequest{
			Token:     token.GetName(),
			UserEmail: token.GetUser(),
			Expires:   token.Expiry(),
			Upsert:    req.Upsert,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		req.Owner = token.GetUser()
		// do not print token here as a security precaution
		r.Printf("Created token for user %q\n", token.GetUser())
	case storage.KindLogForwarder:
		forwarder, err := storage.GetLogForwarderMarshaler().Unmarshal(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		err = forwarder.CheckAndSetDefaults()
		if err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.CreateLogForwarder(ctx, req.SiteKey, forwarder)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		if trace.IsAlreadyExists(err) {
			if !req.Upsert {
				return trace.Wrap(err)
			}
			err := r.Operator.UpdateLogForwarder(ctx, req.SiteKey, forwarder)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		r.Printf("Created log forwarder %q\n", forwarder.GetName())
	case storage.KindTLSKeyPair:
		keyPair, err := storage.UnmarshalTLSKeyPair(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := keyPair.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		_, err = r.Operator.UpdateClusterCertificate(ctx, ops.UpdateCertificateRequest{
			AccountID:   req.AccountID,
			SiteDomain:  req.SiteDomain,
			Certificate: []byte(keyPair.GetCert()),
			PrivateKey:  []byte(keyPair.GetPrivateKey()),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		r.Println("Updated TLS keypair")
	case teleservices.KindClusterAuthPreference:
		r.Println(color.YellowString("Cluster auth preference resource is " +
			"obsolete and will be removed in a future release. Please use " +
			"auth gateway resource instead: https://gravitational.com/gravity/docs/cluster/#configuring-cluster-authentication-gateway."))
		cap, err := teleservices.GetAuthPreferenceMarshaler().Unmarshal(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertClusterAuthPreference(ctx, req.SiteKey, cap); err != nil {
			return trace.Wrap(err)
		}
		r.Println("Updated cluster authentication preference")
	case storage.KindSMTPConfig:
		config, err := storage.UnmarshalSMTPConfig(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := config.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpdateSMTPConfig(ctx, req.SiteKey, config)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Println("Updated cluster SMTP configuration")
	case storage.KindAlert:
		alert, err := storage.UnmarshalAlert(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := alert.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpdateAlert(ctx, req.SiteKey, alert)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Updated monitoring alert %q\n", alert.GetName())
	case storage.KindAlertTarget:
		// Alert recipient can be created only if SMTP settings
		// are present, otherwise it will result into invalid
		// Alertmanager configuration.
		if _, err := r.Operator.GetSMTPConfig(req.SiteKey); err != nil {
			if trace.IsNotFound(err) {
				return trace.BadParameter("alert target can only " +
					"be created when cluster SMTP settings " +
					"are configured, please create SMTP " +
					"resource first: https://gravitational.com/gravity/docs/cluster/#configuring-monitoring")
			}
			return trace.Wrap(err)
		}
		target, err := storage.UnmarshalAlertTarget(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := target.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpdateAlertTarget(ctx, req.SiteKey, target)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Updated monitoring alert target %q\n", target.GetName())
	case storage.KindAuthGateway:
		gw, err := storage.UnmarshalAuthGateway(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpsertAuthGateway(ctx, req.SiteKey, gw)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Println("Updated auth gateway configuration")
	case storage.KindRuntimeEnvironment, storage.KindClusterConfiguration:
		err := r.ClusterOperationHandler.UpdateResource(req)
		return trace.Wrap(err)
	case storage.KindPersistentStorage:
		ps, err := storage.UnmarshalPersistentStorage(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpdatePersistentStorage(ctx, ops.UpdatePersistentStorageRequest{
			SiteKey:  req.SiteKey,
			Resource: ps,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		r.Println("Updated persistent storage configuration")
	case "":
		return trace.BadParameter("missing resource kind")
	default:
		return trace.BadParameter("unsupported resource %q, supported are: %v",
			req.Resource.Kind, modules.GetResources().SupportedResources())
	}
	return nil
}

// GetCollection retrieves a collection of specified resources
func (r *Resources) GetCollection(req resources.ListRequest) (resources.Collection, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	r.Log.Debugf("%s.", req)
	switch req.Kind {
	case teleservices.KindGithubConnector, teleservices.KindAuthConnector:
		if req.Name != "" {
			connector, err := r.Operator.GetGithubConnector(req.SiteKey, req.Name, req.WithSecrets)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &githubCollection{connectors: []teleservices.GithubConnector{connector}}, nil
		}
		connectors, err := r.Operator.GetGithubConnectors(req.SiteKey, req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &githubCollection{connectors: connectors}, nil
	case teleservices.KindUser:
		if req.Name != "" {
			user, err := r.Operator.GetUser(req.SiteKey, req.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &userCollection{users: []teleservices.User{user}}, nil
		}
		users, err := r.Operator.GetUsers(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &userCollection{users: users}, nil
	case storage.KindToken:
		if req.User == "" {
			return nil, trace.BadParameter("please specify user via --user flag")
		}
		// using existing keys API here which is compatible so we don't
		// have to roll out separate tokens API for now
		keys, err := r.Operator.GetAPIKeys(req.User)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tokens := make([]storage.Token, 0, len(keys))
		for _, key := range keys {
			if req.Name != "" && req.Name != key.Token {
				continue
			}
			tokens = append(tokens, storage.NewTokenFromV1(key))
		}
		if req.Name != "" && len(tokens) == 0 {
			return nil, trace.NotFound("token not found")
		}
		return &tokenCollection{tokens: tokens}, nil
	case storage.KindLogForwarder:
		forwarders, err := r.Operator.GetLogForwarders(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var filtered []storage.LogForwarder
		if req.Name != "" {
			for i := range forwarders {
				if forwarders[i].GetName() == req.Name {
					filtered = append(filtered, forwarders[i])
					break
				}
			}
			if len(filtered) == 0 {
				return nil, trace.NotFound("forwarder %q is not found", req.Name)
			}
		} else {
			filtered = forwarders
		}
		return &logForwardersCollection{logForwarders: filtered}, nil
	case storage.KindTLSKeyPair:
		// always ignore name parameter for tls key pairs, because there is only one
		cert, err := r.Operator.GetClusterCertificate(req.SiteKey, req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		keyPair := storage.NewTLSKeyPair(cert.Certificate, cert.PrivateKey)
		return &tlsKeyPairCollection{keyPairs: []storage.TLSKeyPair{keyPair}}, nil
	case teleservices.KindClusterAuthPreference:
		r.Println(color.YellowString("Cluster auth preference resource is " +
			"obsolete and will be removed in a future release. Please use " +
			"auth gateway resource instead: https://gravitational.com/gravity/docs/cluster/#configuring-cluster-authentication-gateway."))
		authPreference, err := r.Operator.GetClusterAuthPreference(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clusterAuthPreferenceCollection{authPreference}, nil
	case storage.KindAuthGateway:
		gw, err := r.Operator.GetAuthGateway(req.SiteKey)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("auth gateway resource not found")
			}
			return nil, trace.Wrap(err)
		}
		return &authGatewayCollection{gw}, nil
	case storage.KindSMTPConfig:
		config, err := r.Operator.GetSMTPConfig(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return smtpConfigCollection{config}, nil
	case storage.KindAlert:
		alerts, err := r.Operator.GetAlerts(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var filtered []storage.Alert
		if req.Name != "" {
			for i := range alerts {
				if alerts[i].GetName() == req.Name {
					filtered = append(filtered, alerts[i])
					break
				}
			}
			if len(filtered) == 0 {
				return nil, trace.NotFound("alert %q is not found", req.Name)
			}
		} else {
			filtered = alerts
		}
		return alertCollection(filtered), nil
	case storage.KindAlertTarget:
		alertTargets, err := r.Operator.GetAlertTargets(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return alertTargetCollection(alertTargets), nil
	case storage.KindRuntimeEnvironment:
		env, err := r.Operator.GetClusterEnvironmentVariables(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return envCollection{env: env}, nil
	case storage.KindClusterConfiguration:
		config, err := r.Operator.GetClusterConfiguration(req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return configCollection{Interface: config}, nil
	case storage.KindPersistentStorage:
		ps, err := r.Operator.GetPersistentStorage(context.TODO(), req.SiteKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return storageCollection{PersistentStorage: ps}, nil
	case storage.KindOperation:
		operations, err := r.Operator.GetSiteOperations(req.SiteKey, ops.OperationsFilter{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var resources []storage.Operation
		for _, op := range operations {
			if req.Name != "" && req.Name != op.ID {
				continue
			}
			resource, err := ops.NewOperation(op)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			resources = append(resources, resource)
		}
		if req.Name != "" && len(resources) == 0 {
			return nil, trace.NotFound("operation %v not found", req.Name)
		}
		return &operationsCollection{operations: resources}, nil
	case "":
		return nil, trace.BadParameter("missing resource kind")
	}
	return nil, trace.BadParameter("unsupported resource %q, supported are: %v",
		req.Kind, modules.GetResources().SupportedResources())
}

// Remove removes the specified resource
func (r *Resources) Remove(ctx context.Context, req resources.RemoveRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	r.Log.Infof("%s.", req)
	switch req.Kind {
	case teleservices.KindGithubConnector:
		if err := r.Operator.DeleteGithubConnector(ctx, req.SiteKey, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Github connector %q has been deleted\n", req.Name)
	case teleservices.KindUser:
		if err := r.Operator.DeleteUser(ctx, req.SiteKey, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("User %q has been deleted\n", req.Name)
	case storage.KindToken:
		user := req.Owner
		if user == "" {
			user = r.CurrentUser
		}
		// using existing keys API here which is compatible so we don't
		// have to roll out separate tokens API for now
		if err := r.Operator.DeleteAPIKey(ctx, user, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Token %q has been deleted for user %q\n", req.Name, user)
	case storage.KindLogForwarder:
		if err := r.Operator.DeleteLogForwarder(ctx, req.SiteKey, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Log forwarder %q has been deleted\n", req.Name)
	case storage.KindTLSKeyPair:
		if err := r.Operator.DeleteClusterCertificate(ctx, req.SiteKey); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("TLS key pair %q has been deleted\n", req.Name)
	case storage.KindSMTPConfig:
		// SMTP configuration can be deleted only if there is no
		// alert recipient configured, otherwise it will result
		// into invalid Alertmanager configuration.
		alertTargets, err := r.Operator.GetAlertTargets(req.SiteKey)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if len(alertTargets) != 0 {
			return trace.BadParameter("SMTP configuration can " +
				"only be deleted if there is no alert target, " +
				"please remove alert target using 'gravity " +
				"resource rm alerttarget' first")
		}
		if err := r.Operator.DeleteSMTPConfig(ctx, req.SiteKey); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Println("SMTP configuration has been deleted")
	case storage.KindAlert:
		if err := r.Operator.DeleteAlert(ctx, req.SiteKey, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Alert %q has been deleted\n", req.Name)
	case storage.KindAlertTarget:
		if err := r.Operator.DeleteAlertTarget(ctx, req.SiteKey); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Println("Alert target has been deleted")
	case storage.KindRuntimeEnvironment, storage.KindClusterConfiguration:
		err := r.ClusterOperationHandler.RemoveResource(req)
		return trace.Wrap(err)
	case "":
		return trace.BadParameter("missing resource kind")
	default:
		return trace.BadParameter("unsupported resource %q, supported are: %v",
			req.Kind, modules.GetResources().SupportedResourcesToRemove())
	}
	return nil
}

// ClusterOperationHandler defines a service to manage resources based on cluster operations
type ClusterOperationHandler interface {
	// RemoveResource removes the specified resource
	RemoveResource(resources.RemoveRequest) error
	// UpdateResource creates or updates the specified resource
	UpdateResource(resources.CreateRequest) error
}

// Validate checks whether the specified resource
// represents a valid resource.
func Validate(resource storage.UnknownResource) (err error) {
	switch resource.Kind {
	case teleservices.KindGithubConnector:
		_, err = teleservices.GetGithubConnectorMarshaler().Unmarshal(resource.Raw)
	case teleservices.KindUser:
		_, err = teleservices.GetUserMarshaler().UnmarshalUser(resource.Raw)
	case storage.KindToken:
		_, err = storage.GetTokenMarshaler().UnmarshalToken(resource.Raw)
	case storage.KindLogForwarder:
		_, err = storage.GetLogForwarderMarshaler().Unmarshal(resource.Raw)
	case storage.KindTLSKeyPair:
		_, err = storage.UnmarshalTLSKeyPair(resource.Raw)
	case teleservices.KindClusterAuthPreference:
		_, err = teleservices.GetAuthPreferenceMarshaler().Unmarshal(resource.Raw)
	case storage.KindSMTPConfig:
		_, err = storage.UnmarshalSMTPConfig(resource.Raw)
	case storage.KindAlert:
		_, err = storage.UnmarshalAlert(resource.Raw)
	case storage.KindAlertTarget:
		_, err = storage.UnmarshalAlertTarget(resource.Raw)
	case storage.KindAuthGateway:
		_, err = storage.UnmarshalAuthGateway(resource.Raw)
	case storage.KindRuntimeEnvironment:
		_, err = storage.UnmarshalEnvironmentVariables(resource.Raw)
	case storage.KindClusterConfiguration:
		config, err := clusterconfig.Unmarshal(resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		globalConfig := config.GetGlobalConfig()
		return validate.KubernetesSubnetsFromStrings(globalConfig.PodCIDR, globalConfig.ServiceCIDR)
	case storage.KindPersistentStorage:
		_, err = storage.UnmarshalPersistentStorage(resource.Raw)
	default:
		return trace.NotImplemented("unsupported resource %q, supported are: %v",
			resource.Kind, modules.GetResources().SupportedResources())
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
