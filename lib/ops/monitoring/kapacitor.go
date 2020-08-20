/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/fatih/color"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// GetAlerts returns all active Kapacitor alerts of the level warning or higher.
//
// The provided HTTP client should be able to resolve K8s service names.
func GetAlerts(httpClient *http.Client) ([]StateResponse, error) {
	kapacitor, err := NewKapacitor(httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	alerts, err := kapacitor.GetAlerts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return alerts, nil
}

// Kapacitor is the Kapactor API client.
type Kapacitor struct {
	// Client is the underlying HTTP client.
	*roundtrip.Client
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

// NewKapacitor returns a new Kapacitor API client.
func NewKapacitor(httpClient *http.Client) (*Kapacitor, error) {
	client, err := roundtrip.NewClient(defaults.KapacitorServiceAddr, "", roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Kapacitor{
		Client:      client,
		FieldLogger: logrus.WithField(trace.Component, "kapacitor"),
	}, nil
}

// GetAlerts returns all active Kapacitor alerts with level warning or higher.
func (k *Kapacitor) GetAlerts() (result []StateResponse, err error) {
	response, err := Get(k.Client, k.Endpoint("/kapacitor/v1/alerts/topics"), url.Values{"min-level": []string{levelWarning}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var topics TopicsResponse
	if err := json.Unmarshal(response.Bytes(), &topics); err != nil {
		return nil, trace.Wrap(err)
	}
	for _, topic := range topics.Topics {
		response, err := Get(k.Client, k.Endpoint(topic.EventsLink.Link), url.Values{"min-level": []string{levelWarning}})
		if err != nil {
			k.WithError(err).Errorf("Failed to retrieve topic events: %v.", topic)
			continue
		}
		var events EventsResponse
		if err := json.Unmarshal(response.Bytes(), &events); err != nil {
			k.WithError(err).Errorf("Failed to unmarshal events response: %v.", string(response.Bytes()))
			continue
		}
		for _, event := range events.Events {
			result = append(result, event.State)
		}
	}
	return result, nil
}

// TopicsResponse contains list of active alerts.
type TopicsResponse struct {
	// Topics is a list of individual alerts.
	Topics []TopicResponse `json:"topics"`
}

// TopicResponse represents an individual alert.
type TopicResponse struct {
	// EventsLink contains link to the alert events.
	EventsLink EventsLinkResponse `json:"events-link"`
}

// EventsLinkResponse contains link to the alert events.
type EventsLinkResponse struct {
	// Link is the URL to the alert events.
	Link string `json:"href"`
}

// EventsResponse contains active alert events.
type EventsResponse struct {
	// Events is a list of alert events.
	Events []EventResponse `json:"events"`
}

// EventResponse contains a single alert event.
type EventResponse struct {
	// State contains event information.
	State StateResponse `json:"state"`
}

// StateResponse contains alert event information.
type StateResponse struct {
	// Message is the event message.
	Message string `json:"message"`
	// Level is the event level.
	Level string `json:"level"`
}

// String format the event so it can be printed to the console.
func (r StateResponse) String() string {
	switch r.Level {
	case levelWarning:
		return color.YellowString("[%v] %v", r.Level, r.Message)
	case levelCritical:
		return color.RedString("[%v] %v", r.Level, r.Message)
	}
	return fmt.Sprintf("[%v] %v", r.Level, r.Message)
}

const (
	// levelWarning is the Kapacitor warning alert level.
	levelWarning = "WARNING"
	// levelCritical is the Kapacitor critical alert level.
	levelCritical = "CRITICAL"
)
