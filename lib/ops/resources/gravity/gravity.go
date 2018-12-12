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
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Resources is a controller that manages cluster local resources
type Resources struct {
	// Config is the controller configuration
	Config
	// cluster is the local cluster
	cluster *ops.Site
}

// Config is gravity resource controller configuration
type Config struct {
	// Operator is local cluster ops client
	Operator ops.Operator
	// CurrentUser is the currently logged in user
	CurrentUser string
	// Silent provides methods for printing
	localenv.Silent
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
	localCluster, err := config.Operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Resources{
		Config:  config,
		cluster: localCluster,
	}, nil
}

// Create creates the provided resource
func (r *Resources) Create(req resources.CreateRequest) error {
	switch req.Resource.Kind {
	case teleservices.KindGithubConnector:
		conn, err := teleservices.GetGithubConnectorMarshaler().Unmarshal(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertGithubConnector(r.cluster.Key(), conn); err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Created Github connector %q\n", conn.GetName())
	case teleservices.KindUser:
		user, err := teleservices.GetUserMarshaler().UnmarshalUser(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertUser(r.cluster.Key(), user); err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Created user %q\n", user.GetName())
	case storage.KindToken:
		token, err := storage.GetTokenMarshaler().UnmarshalToken(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		user := req.User
		if user == "" {
			user = r.CurrentUser
		}
		if token.GetUser() == "" {
			token.SetUser(user)
		}
		if err := token.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		// using existing keys API here which is compatible so we don't
		// have to roll out separate tokens API for now
		_, err = r.Operator.CreateAPIKey(ops.NewAPIKeyRequest{
			Token:     token.GetName(),
			UserEmail: token.GetUser(),
			Expires:   token.Expiry(),
			Upsert:    req.Upsert,
		})
		if err != nil {
			return trace.Wrap(err)
		}
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
		err = r.Operator.CreateLogForwarder(r.cluster.Key(), forwarder)
		if err == nil {
			r.Printf("Created log forwarder %q\n", forwarder.GetName())
			return nil
		}
		if trace.IsAlreadyExists(err) && req.Upsert {
			err := r.Operator.UpdateLogForwarder(r.cluster.Key(), forwarder)
			if err != nil {
				return trace.Wrap(err)
			}
			r.Printf("Updated log forwarder %q\n", forwarder.GetName())
			return nil
		}
		return trace.Wrap(err)
	case storage.KindTLSKeyPair:
		keyPair, err := storage.UnmarshalTLSKeyPair(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := keyPair.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		_, err = r.Operator.UpdateClusterCertificate(ops.UpdateCertificateRequest{
			AccountID:   r.cluster.AccountID,
			SiteDomain:  r.cluster.Domain,
			Certificate: []byte(keyPair.GetCert()),
			PrivateKey:  []byte(keyPair.GetPrivateKey()),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		r.Println("Updated TLS keypair")
	case teleservices.KindClusterAuthPreference:
		cap, err := teleservices.GetAuthPreferenceMarshaler().Unmarshal(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.Operator.UpsertClusterAuthPreference(r.cluster.Key(), cap); err != nil {
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
		err = r.Operator.UpdateSMTPConfig(r.cluster.Key(), config)
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
		err = r.Operator.UpdateAlert(r.cluster.Key(), alert)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Updated monitoring alert %q\n", alert.GetName())
	case storage.KindAlertTarget:
		target, err := storage.UnmarshalAlertTarget(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := target.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpdateAlertTarget(r.cluster.Key(), target)
		if err != nil {
			return trace.Wrap(err)
		}
		r.Printf("Updated monitoring alert target %q\n", target.GetName())
	case storage.KindEnvironment:
		env, err := storage.UnmarshalEnvironment(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := env.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		err = r.Operator.UpdateClusterEnvironmentVariables(ops.UpdateClusterEnvironmentVariablesRequest{
			Key: r.cluster.Key(),
			Env: env,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		r.Println("Updated cluster environment")
	case "":
		return trace.BadParameter("missing resource kind")
	default:
		return trace.BadParameter("unsupported resource %q, supported are: %v",
			req.Resource.Kind, modules.Get().SupportedResources())
	}
	return nil
}

// GetCollection retrieves a collection of specified resources
func (r *Resources) GetCollection(req resources.ListRequest) (resources.Collection, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	switch req.Kind {
	case teleservices.KindGithubConnector, teleservices.KindAuthConnector, "auth":
		if req.Name != "" {
			connector, err := r.Operator.GetGithubConnector(r.cluster.Key(), req.Name, req.WithSecrets)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &githubCollection{connectors: []teleservices.GithubConnector{connector}}, nil
		}
		connectors, err := r.Operator.GetGithubConnectors(r.cluster.Key(), req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &githubCollection{connectors: connectors}, nil
	case teleservices.KindUser, "users":
		if req.Name != "" {
			user, err := r.Operator.GetUser(r.cluster.Key(), req.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &userCollection{users: []teleservices.User{user}}, nil
		}
		users, err := r.Operator.GetUsers(r.cluster.Key())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &userCollection{users: users}, nil
	case storage.KindToken, "tokens":
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
	case storage.KindLogForwarder, "logforwarders":
		forwarders, err := r.Operator.GetLogForwarders(r.cluster.Key())
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
	case storage.KindTLSKeyPair, "tlskeypairs", "tls":
		// always ignore name parameter for tls key pairs, because there is only one
		cert, err := r.Operator.GetClusterCertificate(r.cluster.Key(), req.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		keyPair := storage.NewTLSKeyPair(cert.Certificate, cert.PrivateKey)
		return &tlsKeyPairCollection{keyPairs: []storage.TLSKeyPair{keyPair}}, nil
	case teleservices.KindClusterAuthPreference, "authpreference", "cap":
		authPreference, err := r.Operator.GetClusterAuthPreference(r.cluster.Key())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clusterAuthPreferenceCollection{authPreference}, nil
	case storage.KindSMTPConfig, "smtps":
		config, err := r.Operator.GetSMTPConfig(r.cluster.Key())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return smtpConfigCollection{config}, nil
	case storage.KindAlert, "alerts":
		alerts, err := r.Operator.GetAlerts(r.cluster.Key())
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
	case storage.KindAlertTarget, "alerttargets":
		alertTargets, err := r.Operator.GetAlertTargets(r.cluster.Key())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return alertTargetCollection(alertTargets), nil
	case storage.KindEnvironment, "environments", "env":
		// always ignore name parameter for tls key pairs, because there is only one
		env, err := r.Operator.GetClusterEnvironmentVariables(r.cluster.Key())
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		return envCollection{env: env}, nil
	}
	return nil, trace.BadParameter("unsupported resource %q, supported are: %v",
		req.Kind, modules.Get().SupportedResources())
}

// Remove removes the specified resource
func (r *Resources) Remove(req resources.RemoveRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	switch req.Kind {
	case teleservices.KindGithubConnector:
		if err := r.Operator.DeleteGithubConnector(r.cluster.Key(), req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Github connector %q has been deleted\n", req.Name)
	case teleservices.KindUser, "users":
		if err := r.Operator.DeleteUser(r.cluster.Key(), req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("User %q has been deleted\n", req.Name)
	case storage.KindToken, "tokens":
		user := req.User
		if user == "" {
			user = r.CurrentUser
		}
		// using existing keys API here which is compatible so we don't
		// have to roll out separate tokens API for now
		if err := r.Operator.DeleteAPIKey(user, req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Token %q has been deleted for user %q\n", req.Name, user)
	case storage.KindLogForwarder, "logforwarders":
		if err := r.Operator.DeleteLogForwarder(r.cluster.Key(), req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Log forwarder %q has been deleted\n", req.Name)
	case storage.KindTLSKeyPair, "tlskeypairs", "tls":
		if err := r.Operator.DeleteClusterCertificate(r.cluster.Key()); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("TLS key pair %q has been deleted\n", req.Name)
	case storage.KindSMTPConfig, "smtps":
		if err := r.Operator.DeleteSMTPConfig(r.cluster.Key()); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Println("SMTP configuration has been deleted")
	case storage.KindAlert, "alerts":
		if err := r.Operator.DeleteAlert(r.cluster.Key(), req.Name); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Printf("Alert %q has been deleted\n", req.Name)
	case storage.KindAlertTarget, "alerttargets":
		if err := r.Operator.DeleteAlertTarget(r.cluster.Key()); err != nil {
			if trace.IsNotFound(err) && req.Force {
				return nil
			}
			return trace.Wrap(err)
		}
		r.Println("Alert target has been deleted")
	default:
		return trace.BadParameter("unsupported resource %q, supported are: %v",
			req.Kind, modules.Get().SupportedResourcesToRemove())
	}
	return nil
}
