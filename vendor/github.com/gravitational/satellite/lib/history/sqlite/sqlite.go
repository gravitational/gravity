/*
Copyright 2019 Gravitational, Inc.

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

// Package sqlite provides Timeline implementation backed by a SQLite database.
package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/lib/history"
	"github.com/gravitational/satellite/utils"

	"github.com/gravitational/trace"
	"github.com/jmoiron/sqlx"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// Config defines Timeline configuration.
type Config struct {
	// DBPath specifies the database location.
	DBPath string
	// RetentionDuration specifies the duration to store events.
	RetentionDuration time.Duration
	// Clock will be used to record event timestamps.
	Clock clockwork.Clock
}

// CheckAndSetDefaults validates this configuration object.
// Config values that were not specified will be set to their default values if
// available.
func (c *Config) CheckAndSetDefaults() error {
	var errors []error

	if c.DBPath == "" {
		errors = append(errors, trace.BadParameter("sqlite database path must be provided"))
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.RetentionDuration == time.Duration(0) {
		c.RetentionDuration = defaultTimelineRentention
	}

	return trace.NewAggregate(errors...)
}

// Timeline represents a timeline of status events.
// Timeline events are stored in a local sqlite database.
// The timeline will retain events for a specified duration and then deleted.
//
// Implements history.Timeline
type Timeline struct {
	// config contains timeline configuration.
	config Config
	// database points to underlying sqlite database.
	database *sqlx.DB
}

// NewTimeline initializes and returns a new Timeline with the
// specified configuration.
func NewTimeline(ctx context.Context, config Config) (*Timeline, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	timeline := &Timeline{config: config}

	if err := timeline.initSQLite(ctx); err != nil {
		return nil, trace.Wrap(err, "failed to initialize sqlite database")
	}

	go timeline.eventEvictionLoop(context.TODO())

	return timeline, nil
}

// initSQLite initializes connection to database and initializes `events` table.
func (t *Timeline) initSQLite(ctx context.Context) error {
	dir := filepath.Dir(t.config.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return trace.ConvertSystemError(err)
	}

	database, err := sqlx.ConnectContext(ctx, "sqlite3", t.config.DBPath)
	if err != nil {
		return trace.Wrap(err, "failed to connect to sqlite database at %s", t.config.DBPath)
	}

	if _, err := database.ExecContext(ctx, createTableEvents); err != nil {
		return trace.Wrap(err, "failed to create sqlite tables")
	}

	t.database = database
	return nil
}

// eventEvictionLoop periodically evicts old events to free up storage.
func (t *Timeline) eventEvictionLoop(ctx context.Context) {
	ticker := t.config.Clock.NewTicker(evictionFrequency)
	defer ticker.Stop()
	for range ticker.Chan() {
		if utils.IsContextDone(ctx) {
			log.Info("Eviction loop is stopping.")
			return
		}

		ctxEvict, cancel := context.WithTimeout(ctx, evictionTimeout)
		if err := t.evictEvents(ctxEvict, t.getRetentionCutOff()); err != nil {
			log.WithError(err).Warnf("Error evicting expired events.")
		}
		cancel()
	}
}

// evictEvents deletes events that have outlived the timeline retention
// duration. All events before this cut off time will be deleted.
func (t *Timeline) evictEvents(ctx context.Context, retentionCutOff time.Time) (err error) {
	if _, err := t.database.ExecContext(ctx, deleteOldFromEvents, retentionCutOff); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getRetentionCutOff returns the retention cut off time for the timeline. All
// events before this time is expired and should be removed from the timeline.
func (t *Timeline) getRetentionCutOff() time.Time {
	return t.config.Clock.Now().Add(-(t.config.RetentionDuration))
}

// RecordEvents records the provided events into the timeline.
// Duplicate events will be ignored.
func (t *Timeline) RecordEvents(ctx context.Context, events []*pb.TimelineEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Filter out expired events.
	filtered := []*pb.TimelineEvent{}
	for _, event := range events {
		if event.GetTimestamp().ToTime().Before(t.getRetentionCutOff()) {
			log.WithField("filtered-event", event).Debug("Event filtered.")
			continue
		}
		filtered = append(filtered, event)
	}

	if err := t.insertEvents(ctx, filtered); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// insertEvents inserts the provided events into the timeline.
func (t *Timeline) insertEvents(ctx context.Context, events []*pb.TimelineEvent) error {
	sqlExecer := newSQLExecer(t.database)
	for _, event := range events {
		if err := t.insertEvent(ctx, sqlExecer, event); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (t *Timeline) insertEvent(ctx context.Context, execer history.Execer, event *pb.TimelineEvent) error {
	row, err := newDataInserter(event)
	if err != nil {
		log.WithError(err).Warn("Attempting to insert unknown event.")
		return nil
	}

	err = row.Insert(ctx, execer)
	// Unique constraint error indicates duplicate row.
	// Just ignore duplicates and continue.
	if isErrConstraintUnique(err) {
		log.WithField("row", row).Debug("Attempting to insert duplicate row.")
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetEvents returns a filtered list of events based on the provided params.
// Events will be return in sorted order by timestamp.
// The filter uses "AND" logic with the params.
func (t *Timeline) GetEvents(ctx context.Context, params map[string]string) (events []*pb.TimelineEvent, err error) {
	query, args := prepareQuery(params)
	rows, err := t.database.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.WithError(err).Error("Failed to close sql rows.")
		}
	}()

	var row sqlEvent
	for rows.Next() {
		if err := rows.StructScan(&row); err != nil {
			return nil, trace.Wrap(err)
		}
		events = append(events, row.ProtoBuf())
	}

	if err := rows.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	return events, nil
}

// prepareQuery prepares a query string and a list of arguments constructed from
// the provided params.
func prepareQuery(params map[string]string) (query string, args []interface{}) {
	var sb strings.Builder
	index := 0

	// Need to filter params beforehand to check if WHERE clause is needed.
	filterParams(params)

	sb.WriteString("SELECT * FROM events ")
	if len(params) == 0 {
		sb.WriteString("ORDER BY timestamp ASC ")
		return sb.String(), args
	}
	sb.WriteString("WHERE ")

	for key, val := range params {
		sb.WriteString(fmt.Sprintf("%s = ? ", key))
		args = append(args, val)
		if index < len(params)-1 {
			sb.WriteString("AND ")
		}
		index++
	}

	sb.WriteString("ORDER BY timestamp ASC ")
	return sb.String(), args
}

// filterParams will filter out unknown query parameters.
func filterParams(params map[string]string) (filtered map[string]string) {
	filtered = make(map[string]string)
	var fields = []string{"type", "node", "probe", "oldState", "newState"}
	for _, key := range fields {
		if val, ok := params[key]; ok {
			filtered[key] = val
		}
	}
	return filtered
}
