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

package sqlite

import (
	"context"
	"database/sql"
	"time"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/lib/history"

	"github.com/gravitational/trace"
	"github.com/jmoiron/sqlx"
)

// sqlEvent defines an sql event row.
type sqlEvent struct {
	// ID specifies sqlite id.
	ID int `db:"id"`
	// Timestamp specifies event timestamp.
	Timestamp time.Time `db:"timestamp"`
	// EventType specifies event type.
	EventType string `db:"type"`
	// Node specifies name of node.
	Node sql.NullString `db:"node"`
	// Probe specifies name of probe.
	Probe sql.NullString `db:"probe"`
	// Old specifies previous probe state.
	Old sql.NullString `db:"oldState"`
	// New specifies new probe state.
	New sql.NullString `db:"newState"`
}

// newDataInserter constructs a new DataInserter from the provided timeline event.
func newDataInserter(event *pb.TimelineEvent) (row history.DataInserter, err error) {
	timestamp := event.GetTimestamp().ToTime()
	switch t := event.GetData().(type) {
	case *pb.TimelineEvent_ClusterRecovered:
		return newClusterRecovered(timestamp), nil
	case *pb.TimelineEvent_ClusterDegraded:
		return newClusterDegraded(timestamp), nil
	case *pb.TimelineEvent_NodeAdded:
		e := event.GetNodeAdded()
		return newNodeAdded(timestamp, e.GetNode()), nil
	case *pb.TimelineEvent_NodeRemoved:
		e := event.GetNodeRemoved()
		return newNodeRemoved(timestamp, e.GetNode()), nil
	case *pb.TimelineEvent_NodeRecovered:
		e := event.GetNodeRecovered()
		return newNodeRecovered(timestamp, e.GetNode()), nil
	case *pb.TimelineEvent_NodeDegraded:
		e := event.GetNodeDegraded()
		return newNodeDegraded(timestamp, e.GetNode()), nil
	case *pb.TimelineEvent_ProbeSucceeded:
		e := event.GetProbeSucceeded()
		return newProbeSucceeded(timestamp, e.GetNode(), e.GetProbe()), nil
	case *pb.TimelineEvent_ProbeFailed:
		e := event.GetProbeFailed()
		return newProbeFailed(timestamp, e.GetNode(), e.GetProbe()), nil
	default:
		return row, trace.BadParameter("unknown event type %T", t)
	}
}

// newProtoBuffer constructs a new ProtoBuffer from the provided sql row.
func newProtoBuffer(row sqlEvent) (event history.ProtoBuffer, err error) {
	switch row.EventType {
	case clusterDegradedType:
		return clusterDegraded{sqlEvent: row}, nil
	case clusterRecoveredType:
		return clusterRecovered{sqlEvent: row}, nil
	case nodeAddedType:
		return nodeAdded{sqlEvent: row}, nil
	case nodeRemovedType:
		return nodeRemoved{sqlEvent: row}, nil
	case nodeDegradedType:
		return nodeDegraded{sqlEvent: row}, nil
	case nodeRecoveredType:
		return nodeRecovered{sqlEvent: row}, nil
	case probeFailedType:
		return probeFailed{sqlEvent: row}, nil
	case probeSucceededType:
		return probeSucceeded{sqlEvent: row}, nil
	default:
		return event, trace.BadParameter("unknown event type %s", row.EventType)
	}
}

// sqlExecer executes sql statements.
type sqlExecer struct {
	db *sqlx.DB
}

// newSQLExecer constructs a new sqlExecer.
func newSQLExecer(db *sqlx.DB) *sqlExecer {
	return &sqlExecer{db: db}
}

// Exec executes the provided stmt with the provided args.
func (e *sqlExecer) Exec(ctx context.Context, stmt string, args ...interface{}) error {
	_, err := e.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// clusterDegraded defines a cluster degraded event.
type clusterDegraded struct {
	sqlEvent
}

// newClusterDegraded constructs a new cluster degraded event.
func newClusterDegraded(timestamp time.Time) clusterDegraded {
	return clusterDegraded{
		sqlEvent: sqlEvent{
			Timestamp: timestamp,
			EventType: clusterDegradedType,
		},
	}
}

// Insert inserts event into storage using the provided execer.
func (e clusterDegraded) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type) VALUES (?,?)"
	args := []interface{}{e.Timestamp, e.EventType}
	return trace.Wrap(execer.Exec(ctx, insertStmt, args...))
}

// ProtoBuf returns event as a protobuf message.
func (e clusterDegraded) ProtoBuf() (*pb.TimelineEvent, error) {
	return history.NewClusterDegraded(e.Timestamp), nil
}

// clusterRecovered defines a cluster degraded event.
type clusterRecovered struct {
	sqlEvent
}

// newClusterRecovered constructs a new cluster recovered event.
func newClusterRecovered(timestamp time.Time) clusterRecovered {
	return clusterRecovered{
		sqlEvent: sqlEvent{
			Timestamp: timestamp,
			EventType: clusterRecoveredType,
		},
	}
}

// Insert inserts event into storage using the provided execer.
func (e clusterRecovered) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type) VALUES (?,?)"
	args := []interface{}{e.Timestamp, e.EventType}
	return trace.Wrap(execer.Exec(ctx, insertStmt, args...))
}

// ProtoBuf returns event as a protobuf message.
func (e clusterRecovered) ProtoBuf() (*pb.TimelineEvent, error) {
	return history.NewClusterRecovered(e.Timestamp), nil
}

// nodeAdded defines a node added event.
type nodeAdded struct {
	sqlEvent
}

// newNodeAdded constructs a new node added event.
func newNodeAdded(timestamp time.Time, node string) nodeAdded {
	return nodeAdded{
		sqlEvent: sqlEvent{
			Timestamp: timestamp,
			EventType: nodeAddedType,
			Node:      sql.NullString{String: node, Valid: true},
		},
	}
}

// Insert inserts event into storage using the provided execer.
func (e nodeAdded) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node) VALUES (?,?,?)"
	args := []interface{}{e.Timestamp, e.EventType, e.Node.String}
	return trace.Wrap(execer.Exec(ctx, insertStmt, args...))
}

// ProtoBuf returns event as a protobuf message.
func (e nodeAdded) ProtoBuf() (*pb.TimelineEvent, error) {
	return history.NewNodeAdded(e.Timestamp, e.Node.String), nil
}

// nodeRemoved defines a node added event.
type nodeRemoved struct {
	sqlEvent
}

// newNodeRemoved constructs a new node removed event.
func newNodeRemoved(timestamp time.Time, node string) nodeRemoved {
	return nodeRemoved{
		sqlEvent: sqlEvent{
			Timestamp: timestamp,
			EventType: nodeRemovedType,
			Node:      sql.NullString{String: node, Valid: true},
		},
	}
}

// Insert inserts event into storage using the provided execer.
func (e nodeRemoved) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node) VALUES (?,?,?)"
	args := []interface{}{e.Timestamp, e.EventType, e.Node.String}
	return trace.Wrap(execer.Exec(ctx, insertStmt, args...))
}

// ProtoBuf returns event as a protobuf message.
func (e nodeRemoved) ProtoBuf() (*pb.TimelineEvent, error) {
	return history.NewNodeRemoved(e.Timestamp, e.Node.String), nil
}

// nodeDegraded defines a node degraded event.
type nodeDegraded struct {
	sqlEvent
}

// newNodeDegraded constructs a new node degraded event.
func newNodeDegraded(timestamp time.Time, node string) nodeDegraded {
	return nodeDegraded{
		sqlEvent: sqlEvent{
			Timestamp: timestamp,
			EventType: nodeDegradedType,
			Node:      sql.NullString{String: node, Valid: true},
		},
	}
}

// Insert inserts event into storage using the provided execer.
func (e nodeDegraded) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node) VALUES (?,?,?)"
	args := []interface{}{e.Timestamp, e.EventType, e.Node.String}
	return trace.Wrap(execer.Exec(ctx, insertStmt, args...))
}

// ProtoBuf returns event as a protobuf message.
func (e nodeDegraded) ProtoBuf() (*pb.TimelineEvent, error) {
	return history.NewNodeDegraded(e.Timestamp, e.Node.String), nil
}

// nodeRecovered defines a node recovered event.
type nodeRecovered struct {
	sqlEvent
}

// newNodeRecovered constructs a new node recovered event.
func newNodeRecovered(timestamp time.Time, node string) nodeRecovered {
	return nodeRecovered{
		sqlEvent: sqlEvent{
			Timestamp: timestamp,
			EventType: nodeRecoveredType,
			Node:      sql.NullString{String: node, Valid: true},
		},
	}
}

// Insert inserts event into storage using the provided execer.
func (e nodeRecovered) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node) VALUES (?,?,?)"
	args := []interface{}{e.Timestamp, e.EventType, e.Node.String}
	return trace.Wrap(execer.Exec(ctx, insertStmt, args...))
}

// ProtoBuf returns event as a protobuf message.
func (e nodeRecovered) ProtoBuf() (*pb.TimelineEvent, error) {
	return history.NewNodeRecovered(e.Timestamp, e.Node.String), nil
}

// probeFailed defines a probe failed event.
type probeFailed struct {
	sqlEvent
}

// newProbeFailed constructs a new probe failed event.
func newProbeFailed(timestamp time.Time, node, probe string) probeFailed {
	return probeFailed{
		sqlEvent: sqlEvent{
			Timestamp: timestamp,
			EventType: probeFailedType,
			Node:      sql.NullString{String: node, Valid: true},
			Probe:     sql.NullString{String: probe, Valid: true},
		},
	}
}

// Insert inserts event into storage using the provided execer.
func (e probeFailed) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node, probe) VALUES (?,?,?,?)"
	args := []interface{}{e.Timestamp, e.EventType, e.Node.String, e.Probe.String}
	return trace.Wrap(execer.Exec(ctx, insertStmt, args...))
}

// ProtoBuf returns event as a protobuf message.
func (e probeFailed) ProtoBuf() (*pb.TimelineEvent, error) {
	return history.NewProbeFailed(e.Timestamp, e.Node.String, e.Probe.String), nil
}

// probeSucceeded defines a probe succeeded event.
type probeSucceeded struct {
	sqlEvent
}

// newProbeSucceeded constrcuts a new probe succeeded event.
func newProbeSucceeded(timestamp time.Time, node, probe string) probeSucceeded {
	return probeSucceeded{
		sqlEvent: sqlEvent{
			Timestamp: timestamp,
			EventType: probeSucceededType,
			Node:      sql.NullString{String: node, Valid: true},
			Probe:     sql.NullString{String: probe, Valid: true},
		},
	}
}

// Insert inserts event into storage using the provided execer.
func (e probeSucceeded) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node, probe) VALUES (?,?,?,?)"
	args := []interface{}{e.Timestamp, e.EventType, e.Node.String, e.Probe.String}
	return trace.Wrap(execer.Exec(ctx, insertStmt, args...))
}

// ProtoBuf returns event as a protobuf message.
func (e probeSucceeded) ProtoBuf() (*pb.TimelineEvent, error) {
	return history.NewProbeSucceeded(e.Timestamp, e.Node.String, e.Probe.String), nil
}
