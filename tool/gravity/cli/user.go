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

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
)

func createUser(localEnv *localenv.LocalEnvironment, opsCenterURL, username, type_, password string) error {
	operator, err := localEnv.OperatorService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	err = operator.CreateUser(ops.NewUserRequest{
		Name:     username,
		Type:     type_,
		Password: password,
	})

	if err != nil {
		return trace.Wrap(err)
	}

	localEnv.Printf("user %v created\n", username)
	return nil
}

func deleteUser(localEnv *localenv.LocalEnvironment, opsCenterURL, username string) error {
	operator, err := localEnv.OperatorService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := operator.DeleteLocalUser(username); err != nil {
		return trace.Wrap(err)
	}

	localEnv.Printf("user %v deleted\n", username)
	return nil
}

func createAPIKey(localEnv *localenv.LocalEnvironment, opsCenterURL, username string) error {
	operator, err := localEnv.OperatorService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := operator.CreateAPIKey(context.Background(), ops.NewAPIKeyRequest{
		UserEmail: username,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Print(key.Token)
	return nil
}

func getAPIKeys(localEnv *localenv.LocalEnvironment, opsCenterURL, username string) error {
	operator, err := localEnv.OperatorService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	keys, err := operator.GetAPIKeys(username)
	if err != nil {
		return trace.Wrap(err)
	}

	// output all api keys in a table
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "key\texpires\n")
	for _, k := range keys {
		fmt.Fprintf(w, "%v\t%v\n", k.Token, k.Expires)
	}
	w.Flush()
	return nil
}

func deleteAPIKey(localEnv *localenv.LocalEnvironment, opsCenterURL, username, token string) error {
	operator, err := localEnv.OperatorService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := operator.DeleteAPIKey(context.Background(), username, token); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("api key deleted\n")
	return nil
}

func resetUser(env *localenv.LocalEnvironment, email string, ttl time.Duration) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	resetToken, err := operator.CreateUserReset(context.TODO(), ops.CreateUserResetRequest{
		SiteKey: cluster.Key(),
		Name:    email,
		TTL:     ttl,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Password reset token has been created and is valid for %v. Share this URL with the user:\n%v\n\nNOTE: make sure this URL is accessible!\n",
		ttl.String(), resetToken.URL)

	return nil
}

func inviteUser(env *localenv.LocalEnvironment, username string, roles []string, ttl time.Duration) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	inviteToken, err := operator.CreateUserInvite(context.TODO(), ops.CreateUserInviteRequest{
		SiteKey: cluster.Key(),
		Name:    username,
		Roles:   utils.FlattenStringSlice(roles),
		TTL:     ttl,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Signup token has been created and is valid for %v. Share this URL with the user:\n%v\n\nNOTE: make sure this URL is accessible!\n",
		ttl.String(), inviteToken.URL)

	return nil
}
