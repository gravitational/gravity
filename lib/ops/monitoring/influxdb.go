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

package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type influxDB struct {
	*roundtrip.Client

	kubeClient *kubernetes.Clientset
}

// NewInfluxDB returns a new InfluxDB monitoring provider
func NewInfluxDB(kubeClient *kubernetes.Clientset) (Monitoring, error) {
	return &influxDB{kubeClient: kubeClient}, nil
}

func newInfluxDBClient(kclient corev1.CoreV1Interface) (*roundtrip.Client, error) {
	authParam, err := getInfluxDBCredentials(kclient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := roundtrip.NewClient(defaults.InfluxDBAddr(), "", authParam)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = verifyAuth(client); err == nil {
		return client, nil
	}
	if !trace.IsAccessDenied(err) {
		return nil, trace.Wrap(err)
	}
	return backwardsCompatibleClient()
}

func verifyAuth(client *roundtrip.Client) error {
	response, err := Get(client, client.Endpoint("query"), url.Values{"q": []string{showQuery}})
	if err != nil {
		return trace.Wrap(err)
	}
	if response.Code() == http.StatusUnauthorized {
		return trace.AccessDenied("auth failed")
	}
	return nil
}

func backwardsCompatibleClient() (*roundtrip.Client, error) {
	client, err := roundtrip.NewClient(defaults.InfluxDBAddr(), "",
		roundtrip.BasicAuth(
			defaults.InfluxDBAdminUser,
			defaults.InfluxDBAdminPassword,
		))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// Get performs a GET HTTP request to the specified endpoint using the given client.
// Any errors are automatically converted to trace type space upon return
func Get(client *roundtrip.Client, endpoint string, params url.Values) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(client.Get(endpoint, params))
}

// GetRetentionPolicies returns a list of retention policies for the site
func (i *influxDB) GetRetentionPolicies() ([]RetentionPolicy, error) {
	if i.Client == nil {
		var err error
		i.Client, err = newInfluxDBClient(i.kubeClient.CoreV1())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	response, err := Get(i.Client, i.Endpoint("query"), url.Values{"q": []string{showQuery}})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var parsed influxDBResponse
	err = json.Unmarshal(response.Bytes(), &parsed)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(parsed.Results) == 0 {
		return nil, trace.NotFound("results are empty: %v", parsed)
	}

	if len(parsed.Results[0].Series) == 0 {
		return nil, trace.NotFound("series are empty: %v", parsed)
	}

	var policies []RetentionPolicy

	for _, values := range parsed.Results[0].Series[0].Values {
		// each "row" in "show retention policies" response should contain 5 "columns",
		// for us only the first (policy name) and the second (policy duration) are relevant
		if len(values) != 5 {
			return nil, trace.BadParameter("expected 5-element values: %v", parsed)
		}
		name, ok := values[0].(string)
		if !ok {
			return nil, trace.BadParameter("expected first value to be string: %v", parsed)
		}
		durationS, ok := values[1].(string)
		if !ok {
			return nil, trace.BadParameter("expected second value to be string: %v", parsed)
		}
		duration, err := time.ParseDuration(durationS)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse duration: %v", parsed)
		}
		policies = append(policies, RetentionPolicy{
			Name:     name,
			Duration: duration,
		})
	}

	if len(policies) == 0 {
		return nil, trace.NotFound("no retention policies found")
	}

	return policies, nil
}

// UpdateRetentionPolicy configures metrics retention policy
func (i *influxDB) UpdateRetentionPolicy(policy RetentionPolicy) error {
	if i.Client == nil {
		var err error
		i.Client, err = newInfluxDBClient(i.kubeClient.CoreV1())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	_, err := i.PostForm(i.Endpoint("query"), url.Values{
		"q": []string{fmt.Sprintf(updateQuery, policy.Name, policy.Duration.Hours())},
	})
	return trace.Wrap(err)
}

// PostForm is like roundtrip.Client.PostForm but converts returned HTTP errors into trace errors
func (i *influxDB) PostForm(endpoint string, params url.Values) (*roundtrip.Response, error) {
	if i.Client == nil {
		var err error
		i.Client, err = newInfluxDBClient(i.kubeClient.CoreV1())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return httplib.ConvertResponse(i.Client.PostForm(endpoint, params))
}

// influxDBResponse represents a response from InfluxDB API
type influxDBResponse struct {
	// Results is the list of results
	Results []influxDBResult `json:"results"`
}

// influxDBResult represents a single result in InfluxDB API response
type influxDBResult struct {
	// Series is the list of series
	Series []influxDBSeries `json:"series"`
}

// influxDBSeries represents a single series in InfluxDB API result
type influxDBSeries struct {
	// Columns is the list of columns in this series, length should be
	// equals to the length of each slice in values
	Columns []string `json:"columns"`
	// Values is the list of values in this series; values may be of different
	// types hence interface
	Values [][]interface{} `json:"values"`
}

var (
	// showQuery is InfluxDB query to list retention policies
	showQuery = "show retention policies on k8s"
	// updateQuery is InfluxDB query to update retention policy
	updateQuery = "alter retention policy %v on k8s duration %vh"
)
