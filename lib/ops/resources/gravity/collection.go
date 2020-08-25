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
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
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
	common.PrintTableHeader(t, []string{"Name", "Group", "Formula", "Delay", "Labels"})
	for _, alert := range r {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\t%v\n",
			alert.GetName(),
			formatAlertGroup(alert),
			strings.TrimSpace(alert.GetFormula()),
			formatAlertDelay(alert),
			formatAlertLabels(alert))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func formatAlertGroup(alert storage.Alert) string {
	group := alert.GetGroupName()
	if group != "" {
		return group
	}
	return "-"
}

func formatAlertDelay(alert storage.Alert) string {
	delay := alert.GetDelay()
	if delay != 0 {
		return delay.String()
	}
	return "-"
}

func formatAlertLabels(alert storage.Alert) (result string) {
	if len(alert.GetLabels()) == 0 {
		return "-"
	}
	var labels []string
	for k, v := range alert.GetLabels() {
		labels = append(labels, fmt.Sprintf("%v: %v", k, v))
	}
	return strings.Join(labels, ", ")
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
func (c alertCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c {
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
func (c alertTargetCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

type alertTargetCollection []storage.AlertTarget

type authGatewayCollection struct {
	item storage.AuthGateway
}

// Resources returns the resources collection in the generic format
func (c *authGatewayCollection) Resources() ([]teleservices.UnknownResource, error) {
	resource, err := utils.ToUnknownResource(c.item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []teleservices.UnknownResource{*resource}, nil
}

// WriteText serializes auth gateway config in human-friendly text format
func (c *authGatewayCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Parameter", "Value"})
	fmt.Fprintf(t, "Max Connections:\t%v\n", c.item.GetMaxConnections())
	fmt.Fprintf(t, "Max Users:\t%v\n", c.item.GetMaxUsers())
	if c.item.GetClientIdleTimeout() != nil {
		fmt.Fprintf(t, "Client Idle Timeout:\t%v\n", c.item.GetClientIdleTimeout().Value())
	} else {
		fmt.Fprintf(t, "Client Idle Timeout:\tnever\n")
	}
	if c.item.GetDisconnectExpiredCert() != nil {
		fmt.Fprintf(t, "Disconnect Expired Cert:\t%v\n", c.item.GetDisconnectExpiredCert().Value())
	} else {
		fmt.Fprintf(t, "Disconnect Expired Cert:\tno\n")
	}
	if auth := c.item.GetAuthentication(); auth != nil {
		fmt.Fprintf(t, "Authentication:\ttype: %v, second factor: %v\n", auth.Type, auth.SecondFactor)
	}
	fmt.Fprintf(t, "SSH Public Addrs:\t%v\n", formatList(c.item.GetSSHPublicAddrs()))
	fmt.Fprintf(t, "Kubernetes Public Addrs:\t%v\n", formatList(c.item.GetKubernetesPublicAddrs()))
	fmt.Fprintf(t, "Web Public Addrs:\t%v\n", formatList(c.item.GetWebPublicAddrs()))
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func formatList(list []string) string {
	if len(list) == 0 {
		return "-"
	}
	return strings.Join(list, ", ")
}

// WriteJSON serializes collection into JSON format
func (c *authGatewayCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

// WriteYAML serializes collection into YAML format
func (c *authGatewayCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

// ToMarshal returns object that should be marshaled.
func (c *authGatewayCollection) ToMarshal() interface{} {
	return c.item
}

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
	env storage.EnvironmentVariables
}

// WriteText serializes collection in human-friendly text format
func (r configCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintCustomTableHeader(t, []string{"Configuration"}, "=")
	if r.Interface == nil {
		// Empty
		return nil
	}
	if config := r.GetKubeletConfig(); config != nil && len(config.Config) != 0 {
		common.PrintCustomTableHeader(t, []string{"Kubelet"}, "-")
		fmt.Fprintf(t, "%v\n", string(config.Config))
	}
	if config := r.GetGravityControllerServiceConfig(); config != nil {
		common.PrintCustomTableHeader(t, []string{"GravityControllerService"}, "-")
		if len(config.Labels) != 0 {
			fmt.Fprintf(t, "Labels:\t%v\n", formatAnnotations(config.Labels))
		}
		if len(config.Annotations) != 0 {
			fmt.Fprintf(t, "Annotations:\t%v\n", formatAnnotations(config.Annotations))
		}
		fmt.Fprintf(t, "Type:\t%v\n", config.Spec.Type)
		if len(config.Spec.Ports) != 0 {
			fmt.Fprintf(t, "Ports:\t%v\n", config.Spec.Ports)
		}
	}
	config := r.GetGlobalConfig()
	displayCloudConfig := config.CloudProvider != "" || config.CloudConfig != ""
	if displayCloudConfig {
		common.PrintCustomTableHeader(t, []string{"Cloud"}, "-")
		if len(config.CloudProvider) != 0 {
			fmt.Fprintf(t, "Provider:\t%v\n", config.CloudProvider)
		}
		formatCloudConfig(t, config.CloudConfig)
	}
	if len(config.ServiceNodePortRange) != 0 {
		fmt.Fprintf(t, "Service Node Port Range:\t%v\n", config.ServiceNodePortRange)
	}
	if len(config.ProxyPortRange) != 0 {
		fmt.Fprintf(t, "Proxy Port Range:\t%v\n", config.ProxyPortRange)
	}
	if len(config.FeatureGates) != 0 {
		fmt.Fprintf(t, "Feature Gates:\t%v\n", formatFeatureGates(config.FeatureGates))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (r configCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(r, w)
}

// WriteYAML serializes collection into YAML format
func (r configCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(r, w)
}

func (r configCollection) ToMarshal() interface{} {
	return r.Interface
}

// Resources returns the resources collection in the generic format
func (r configCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	resource, err := utils.ToUnknownResource(r.Interface)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resources = append(resources, *resource)
	return resources, nil
}

type configCollection struct {
	clusterconfig.Interface
}

func formatCloudConfig(w io.Writer, config string) {
	if config == "" {
		return
	}
	fmt.Fprintf(w, "Configuration:\n")
	fmt.Fprintf(w, "%v\n", config)
}

func formatFeatureGates(features map[string]bool) string {
	result := make([]string, 0, len(features))
	for feature, enabled := range features {
		result = append(result, fmt.Sprintf("%v=%v", feature, enabled))
	}
	return strings.Join(result, ",")
}

func formatAnnotations(annotations map[string]string) string {
	result := make([]string, 0, len(annotations))
	for key, val := range annotations {
		result = append(result, fmt.Sprintf("%v=%v", key, val))
	}
	return strings.Join(result, ",")
}

type operationsCollection struct {
	operations []storage.Operation
}

// Resources returns the operations collection in the generic format.
func (c *operationsCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.operations {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes operations in human-friendly text format.
func (c *operationsCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"ID", "Description", "State", "Created"})
	for _, o := range c.operations {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\n",
			o.GetName(),
			ops.DescribeOperation(o),
			strings.Title(o.GetState()),
			o.GetCreated().Format(constants.HumanDateFormat))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into json format.
func (c *operationsCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

// ToMarshal returns objects to marshal.
func (c *operationsCollection) ToMarshal() interface{} {
	if len(c.operations) == 1 {
		return c.operations[0]
	}
	return c.operations
}

// WriteYAML serializes collection into yaml format.
func (c *operationsCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

type storageCollection struct {
	storage.PersistentStorage
}

// WriteText serializes collection in human-friendly text format.
func (c storageCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Parameter", "Value"})
	fmt.Fprint(t, "Mount Points:\n")
	fmt.Fprintf(t, "  Exclude:\t%v\n", strings.Join(c.GetMountExcludes(), ", "))
	fmt.Fprint(t, "Vendors:\n")
	fmt.Fprintf(t, "  Include:\t%v\n", strings.Join(c.GetVendorIncludes(), ", "))
	fmt.Fprintf(t, "  Exclude:\t%v\n", strings.Join(c.GetVendorExcludes(), ", "))
	fmt.Fprint(t, "Devices:\n")
	fmt.Fprintf(t, "  Include:\t%v\n", strings.Join(c.GetDeviceIncludes(), ", "))
	fmt.Fprintf(t, "  Exclude:\t%v\n", strings.Join(c.GetDeviceExcludes(), ", "))
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// Resources converts the object to the generic resources collection.
func (c storageCollection) Resources() ([]teleservices.UnknownResource, error) {
	resource, err := utils.ToUnknownResource(c.PersistentStorage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []teleservices.UnknownResource{*resource}, nil
}

// WriteJSON serializes collection into JSON format.
func (c storageCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

// WriteYAML serializes collection into YAML format.
func (c storageCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

// ToMarshal retursn the object to marshal.
func (c storageCollection) ToMarshal() interface{} {
	return c.PersistentStorage
}
