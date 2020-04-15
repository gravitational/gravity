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
//
// Implements history.ProtoBuffer
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

// ProtoBuf returns the sql event row as a protobuf message.
func (r sqlEvent) ProtoBuf() (event *pb.TimelineEvent) {
	switch history.EventType(r.EventType) {
	case history.ClusterDegraded:
		return pb.NewClusterDegraded(r.Timestamp)
	case history.ClusterHealthy:
		return pb.NewClusterHealthy(r.Timestamp)
	case history.NodeAdded:
		return pb.NewNodeAdded(r.Timestamp, r.Node.String)
	case history.NodeRemoved:
		return pb.NewNodeRemoved(r.Timestamp, r.Node.String)
	case history.NodeDegraded:
		return pb.NewNodeDegraded(r.Timestamp, r.Node.String)
	case history.NodeHealthy:
		return pb.NewNodeHealthy(r.Timestamp, r.Node.String)
	case history.ProbeFailed:
		return pb.NewProbeFailed(r.Timestamp, r.Node.String, r.Probe.String)
	case history.ProbeSucceeded:
		return pb.NewProbeSucceeded(r.Timestamp, r.Node.String, r.Probe.String)
	case history.LeaderElected:
		return pb.NewLeaderElected(r.Timestamp, r.Old.String, r.New.String)
	default:
		return pb.NewUnknownEvent(r.Timestamp)
	}
}

// sqlExecer executes sql statements.
//
// Implements history.Execer
type sqlExecer struct {
	db *sqlx.DB
}

// newSQLExecer constructs a new sqlExecer with the provided database.
func newSQLExecer(db *sqlx.DB) *sqlExecer {
	return &sqlExecer{db: db}
}

// Exec executes the provided stmt with the provided args.
func (r *sqlExecer) Exec(ctx context.Context, stmt string, args ...interface{}) error {
	_, err := r.db.ExecContext(ctx, stmt, args...)
	return trace.Wrap(err)
}

// newDataInserter returns the event as a history.DataInserter.
func newDataInserter(event *pb.TimelineEvent) (row history.DataInserter, err error) {
	switch data := event.GetData().(type) {
	case *pb.TimelineEvent_ClusterDegraded:
		return &clusterDegraded{ts: event.GetTimestamp()}, nil
	case *pb.TimelineEvent_ClusterHealthy:
		return &clusterHealthy{ts: event.GetTimestamp()}, nil
	case *pb.TimelineEvent_NodeAdded:
		return &nodeAdded{ts: event.GetTimestamp(), data: data.NodeAdded}, nil
	case *pb.TimelineEvent_NodeRemoved:
		return &nodeRemoved{ts: event.GetTimestamp(), data: data.NodeRemoved}, nil
	case *pb.TimelineEvent_NodeHealthy:
		return &nodeHealthy{ts: event.GetTimestamp(), data: data.NodeHealthy}, nil
	case *pb.TimelineEvent_NodeDegraded:
		return &nodeDegraded{ts: event.GetTimestamp(), data: data.NodeDegraded}, nil
	case *pb.TimelineEvent_ProbeSucceeded:
		return &probeSucceeded{ts: event.GetTimestamp(), data: data.ProbeSucceeded}, nil
	case *pb.TimelineEvent_ProbeFailed:
		return &probeFailed{ts: event.GetTimestamp(), data: data.ProbeFailed}, nil
	case *pb.TimelineEvent_LeaderElected:
		return &leaderElected{ts: event.GetTimestamp(), data: data.LeaderElected}, nil
	default:
		return row, trace.BadParameter("unknown event type %T", data)
	}
}

// clusterDegraded represents a cluster degraded event.
//
// Implements history.DataInserter.
type clusterDegraded struct {
	ts *pb.Timestamp
}

func (r *clusterDegraded) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type) VALUES (?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt, r.ts.ToTime(), history.ClusterDegraded))
}

// clusterHealthy represents a cluster healthy event.
//
// Implements history.DataInserter.
type clusterHealthy struct {
	ts *pb.Timestamp
}

func (r *clusterHealthy) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type) VALUES (?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt, r.ts.ToTime(), history.ClusterHealthy))
}

// nodeAdded represents a node added event.
//
// Implements history.DataInserter.
type nodeAdded struct {
	ts   *pb.Timestamp
	data *pb.NodeAdded
}

func (r *nodeAdded) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node) VALUES (?,?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt, r.ts.ToTime(), history.NodeAdded, r.data.GetNode()))
}

// nodeRemoved represents a node removed event.
//
// Implements history.DataInserter.
type nodeRemoved struct {
	ts   *pb.Timestamp
	data *pb.NodeRemoved
}

func (r *nodeRemoved) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node) VALUES (?,?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt, r.ts.ToTime(), history.NodeRemoved, r.data.GetNode()))
}

// nodeDegraded represents a node degraded event.
//
// Implements history.DataInserter.
type nodeDegraded struct {
	ts   *pb.Timestamp
	data *pb.NodeDegraded
}

func (r *nodeDegraded) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node) VALUES (?,?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt, r.ts.ToTime(), history.NodeDegraded, r.data.GetNode()))
}

// nodeHealthy represents a node healthy event.
//
// Implements history.DataInserter.
type nodeHealthy struct {
	ts   *pb.Timestamp
	data *pb.NodeHealthy
}

func (r *nodeHealthy) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node) VALUES (?,?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt, r.ts.ToTime(), history.NodeHealthy, r.data.GetNode()))
}

// probeFailed represents a probe failed event.
//
// Implements history.DataInserter.
type probeFailed struct {
	ts   *pb.Timestamp
	data *pb.ProbeFailed
}

func (r *probeFailed) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node, probe) VALUES (?,?,?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt,
		r.ts.ToTime(), history.ProbeFailed, r.data.GetNode(), r.data.GetProbe()))
}

// probeSucceeded represents a probe succeeded event.
//
// Implements history.DataInserter.
type probeSucceeded struct {
	ts   *pb.Timestamp
	data *pb.ProbeSucceeded
}

func (r *probeSucceeded) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, node, probe) VALUES (?,?,?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt,
		r.ts.ToTime(), history.ProbeSucceeded, r.data.GetNode(), r.data.GetProbe()))
}

// leaderElected represents a leader elected event.
//
// Implements history.DataInserter.
type leaderElected struct {
	ts   *pb.Timestamp
	data *pb.LeaderElected
}

func (r *leaderElected) Insert(ctx context.Context, execer history.Execer) error {
	const insertStmt = "INSERT INTO events (timestamp, type, oldState, newState) VALUES (?,?,?,?)"
	return trace.Wrap(execer.Exec(ctx, insertStmt,
		r.ts.ToTime(), history.LeaderElected, r.data.GetPrev(), r.data.GetNew()))
}
