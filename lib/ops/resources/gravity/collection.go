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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/buger/goterm"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type githubCollection struct {
	connectors []teleservices.GithubConnector
}

// Resources returns the resources collection in the generic format
func (c *githubCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.connectors {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *githubCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Client ID", "Mapping"})
	for _, conn := range c.connectors {
		fmt.Fprintf(t, "%v\t%v\t%v\n",
			conn.GetName(),
			conn.GetClientID(),
			formatGithubMapping(conn.GetTeamsToLogins()))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func formatGithubMapping(mappings []teleservices.TeamMapping) string {
	var formatted []string
	for _, m := range mappings {
		formatted = append(formatted, fmt.Sprintf("@%v/%v -> %v",
			m.Organization, m.Team, strings.Join(m.Logins, ",")))
	}
	return strings.Join(formatted, "\n")
}

// WriteJSON serializes collection into JSON format
func (c *githubCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

func (c *githubCollection) ToMarshal() interface{} {
	if len(c.connectors) == 1 {
		return c.connectors[0]
	}
	return c.connectors
}

// WriteYAML serializes collection into YAML format
func (c *githubCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

type userCollection struct {
	users []teleservices.User
}

// Resources returns the resources collection in the generic format
func (c *userCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.users {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

type clusterAuthPreferenceCollection []teleservices.AuthPreference

// Resources returns the resources collection in the generic format
func (c clusterAuthPreferenceCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c {
		// teleservices.AuthPreference does not implement teleservices.Resource
		// interface for some reason, so we can't use utils.ToUnknownResource,
		// convert it right here instead
		data, err := json.Marshal(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var resource teleservices.UnknownResource
		err = yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer(data), defaults.DecoderBufferSize).Decode(&resource)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c clusterAuthPreferenceCollection) WriteText(w io.Writer) error {
	if len(c) == 0 {
		return nil
	}

	t := goterm.NewTable(0, 10, 5, ' ', 0)
	cap := c[0] // cannot have more than 1 auth preference
	u2f, _ := cap.GetU2F()
	if u2f != nil {
		common.PrintTableHeader(t, []string{"Type", "Connector Name", "Second Factor", "U2F AppID", "U2F Facets"})
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\t%v\n",
			cap.GetType(),
			cap.GetConnectorName(),
			cap.GetSecondFactor(),
			u2f.AppID,
			strings.Join(u2f.Facets, "; "))
	} else {
		common.PrintTableHeader(t, []string{"Type", "Connector Name", "Second Factor"})
		fmt.Fprintf(t, "%v\t%v\t%v\n",
			cap.GetType(),
			cap.GetConnectorName(),
			cap.GetSecondFactor())
	}

	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c clusterAuthPreferenceCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

// WriteYAML serializes collection into YAML format
func (c clusterAuthPreferenceCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

func (c clusterAuthPreferenceCollection) ToMarshal() interface{} {
	if len(c) == 1 {
		return c[0]
	}
	return c
}

// WriteText serializes collection in human-friendly text format
func (u *userCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Type", "Roles"})
	for _, user := range u.filterUsers() {
		fmt.Fprintf(t, "%v\t%v\t%v\n",
			user.GetName(),
			user.GetType(),
			user.GetRoles())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (u *userCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(u, w)
}

func (u *userCollection) filterUsers() []storage.User {
	var users []storage.User
	for i := range u.users {
		u, ok := (u.users[i]).(storage.User)
		if !ok {
			continue
		}
		// skip some system users
		if utils.StringInSlice([]string{constants.GatekeeperUser, constants.OpsCenterUser}, u.GetName()) {
			continue
		}
		if strings.HasSuffix(u.GetName(), constants.BlobUserSuffix) {
			continue
		}
		users = append(users, u)
	}
	return users
}

func (u *userCollection) ToMarshal() interface{} {
	users := u.filterUsers()
	if len(users) == 1 {
		return users[0]
	}
	return users
}

// WriteYAML serializes collection into YAML format
func (c *userCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

type tokenCollection struct {
	tokens []storage.Token
}

// Resources returns the resources collection in the generic format
func (c *tokenCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.tokens {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *tokenCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Token", "User", "Expires"})
	for _, token := range c.tokens {
		fmt.Fprintf(t, "%v\t%v\t%v\n",
			token.GetName(),
			token.GetUser(),
			formatExpiry(token.Expiry()))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *tokenCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

func (c *tokenCollection) ToMarshal() interface{} {
	if len(c.tokens) == 1 {
		return c.tokens[0]
	}
	return c.tokens
}

// WriteYAML serializes collection into YAML format
func (c *tokenCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

func formatExpiry(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(constants.HumanDateFormat)
}

type logForwardersCollection struct {
	logForwarders []storage.LogForwarder
}

// Resources returns the resources collection in the generic format
func (c *logForwardersCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.logForwarders {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *logForwardersCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Address", "Protocol"})
	for _, forwarder := range c.logForwarders {
		fmt.Fprintf(t, "%v\t%v\t%v\n",
			forwarder.GetName(),
			forwarder.GetAddress(),
			forwarder.GetProtocol())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *logForwardersCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

func (c *logForwardersCollection) ToMarshal() interface{} {
	if len(c.logForwarders) == 1 {
		return c.logForwarders[0]
	}
	return c.logForwarders
}

// WriteYAML serializes collection into YAML format
func (c *logForwardersCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

type tlsKeyPairCollection struct {
	keyPairs []storage.TLSKeyPair
}

// Resources returns the resources collection in the generic format
func (c *tlsKeyPairCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.keyPairs {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *tlsKeyPairCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Common Name", "Organization", "Expires"})
	for _, keyPair := range c.keyPairs {
		var commonName, org string
		info, err := utils.ParseCertificate([]byte(keyPair.GetCert()))
		var validBefore time.Time
		if err != nil {
			commonName = err.Error()
			org = err.Error()
			validBefore = time.Time{}
		} else {
			commonName = info.IssuedTo.CommonName
			org = strings.Join(info.IssuedTo.Organization, ",")
			validBefore = info.Validity.NotAfter
		}
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\n",
			keyPair.GetName(),
			commonName, org,
			validBefore.Format(constants.HumanDateFormat))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *tlsKeyPairCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

// WriteYAML serializes collection into YAML format
func (c *tlsKeyPairCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

func (c *tlsKeyPairCollection) ToMarshal() interface{} {
	if len(c.keyPairs) == 1 {
		return c.keyPairs[0]
	}
	return c.keyPairs
}

// Resources returns the resources collection in the generic format
func (c smtpConfigCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (r smtpConfigCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Host", "Port", "Username"})
	for _, config := range r {
		fmt.Fprintf(t, "%v\t%v\t%v\n", config.GetHost(), config.GetPort(), config.GetUsername())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (r smtpConfigCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(r, w)
}

// WriteYAML serializes collection into YAML format
func (r smtpConfigCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(r, w)
}

func (r smtpConfigCollection) ToMarshal() interface{} {
	if len(r) == 1 {
		return r[0]
	}
	return r
}

type smtpConfigCollection []storage.SMTPConfig

// WriteText serializes collection in human-friendly text format
func (r alertCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Formula"})
	for _, alert := range r {
		fmt.Fprintf(t, "%v\t%v\n", alert.GetName(), alert.GetFormula())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (r alertCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(r, w)
}

// WriteYAML serializes collection into YAML format
func (r alertCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(r, w)
}

func (r alertCollection) ToMarshal() interface{} {
	if len(r) == 1 {
		return r[0]
	}
	return r
}

// Resources returns the resources collection in the generic format
func (r alertCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range r {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

type alertCollection []storage.Alert

// WriteText serializes collection in human-friendly text format
func (r alertTargetCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Email"})
	for _, target := range r {
		fmt.Fprintf(t, "%v\n", target.GetEmail())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (r alertTargetCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(r, w)
}

// WriteYAML serializes collection into YAML format
func (r alertTargetCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(r, w)
}

func (r alertTargetCollection) ToMarshal() interface{} {
	if len(r) == 1 {
		return r[0]
	}
	return r
}

// Resources returns the resources collection in the generic format
func (r alertTargetCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range r {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

type alertTargetCollection []storage.AlertTarget

// WriteText serializes collection in human-friendly text format
func (r envCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Environment"})
	if r.env == nil {
		// Empty
		return nil
	}
	for k, v := range r.env.GetKeyValues() {
		fmt.Fprintf(t, "%v=%v\n", k, v)
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (r envCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(r, w)
}

// WriteYAML serializes collection into YAML format
func (r envCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(r, w)
}

func (r envCollection) ToMarshal() interface{} {
	return r.env
}

// Resources returns the resources collection in the generic format
func (r envCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	resource, err := utils.ToUnknownResource(r.env)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resources = append(resources, *resource)
	return resources, nil
}

type envCollection struct {
	env storage.Environment
}
