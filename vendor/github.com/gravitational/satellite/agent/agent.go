/*
Copyright 2016-2020 Gravitational, Inc.

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

package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gravitational/satellite/agent/cache"
	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/lib/ctxgroup"
	"github.com/gravitational/satellite/lib/history"
	"github.com/gravitational/satellite/lib/history/sqlite"
	"github.com/gravitational/satellite/lib/membership"
	"github.com/gravitational/satellite/lib/rpc/client"
	"github.com/gravitational/satellite/utils"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap/v2"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// Config defines satellite configuration.
type Config struct {
	// Name is the name assigned to this node my Kubernetes.
	Name string

	// RPCAddrs is a list of addresses agent binds to for RPC traffic.
	//
	// Usually, at least two address are used for operation.
	// Localhost is a convenience for local communication.  Cluster-visible
	// IP is required for proper inter-communication between agents.
	RPCAddrs []string

	// CAFile specifies the path to TLS Certificate Authority file
	CAFile string

	// CertFile specifies the path to TLS certificate file
	CertFile string

	// KeyFile specifies the path to TLS certificate key file
	KeyFile string

	// MetricsAddr specifies the address to listen on for web interface and telemetry for Prometheus metrics.
	MetricsAddr string

	// DebugSocketPath specifies the location of the unix domain socket for debug endpoint
	DebugSocketPath string

	// Set of tags for the agent.
	// Tags is a trivial means for adding extra semantic information to an agent.
	Tags map[string]string

	// TimelineConfig specifies sqlite timeline configuration.
	TimelineConfig sqlite.Config

	// Clock to be used for internal time keeping.
	Clock clockwork.Clock

	// Cache is a short-lived storage used by the agent to persist latest health stats.
	cache.Cache

	// DialRPC is a factory function to create clients to other agents.
	client.DialRPC

	// clientCache is a cache of gRPC clients for repeated use.
	clientCache *client.ClientCache

	// Cluster is used to query cluster members.
	membership.Cluster
}

// CheckAndSetDefaults validates this configuration object.
// Config values that were not specified will be set to their default values if
// available.
func (r *Config) CheckAndSetDefaults() error {
	var errors []error
	if r.CAFile == "" {
		errors = append(errors, trace.BadParameter("certificate authority file must be provided"))
	}
	if r.CertFile == "" {
		errors = append(errors, trace.BadParameter("certificate must be provided"))
	}
	if r.KeyFile == "" {
		errors = append(errors, trace.BadParameter("certificate key must be provided"))
	}
	if r.Name == "" {
		errors = append(errors, trace.BadParameter("agent name cannot be empty"))
	}
	if r.Cluster == nil {
		errors = append(errors, trace.BadParameter("cluster membership must be provided"))
	}
	if len(r.RPCAddrs) == 0 {
		errors = append(errors, trace.BadParameter("at least one RPC address must be provided"))
	}
	if len(errors) != 0 {
		return trace.NewAggregate(errors...)
	}
	if r.Tags == nil {
		r.Tags = make(map[string]string)
	}
	if r.Clock == nil {
		r.Clock = clockwork.NewRealClock()
	}

	if r.clientCache == nil {
		r.clientCache = &client.ClientCache{}
	}

	if r.DialRPC == nil {
		r.DialRPC = r.clientCache.DefaultDialRPC(r.CAFile, r.CertFile, r.KeyFile)
	}
	return nil
}

type agent struct {
	// Config is the agent configuration.
	Config

	// ClusterTimeline keeps track of all timeline events in the cluster. This
	// timeline is only used by members that have the role 'master'.
	ClusterTimeline history.Timeline

	// LocalTimeline keeps track of local timeline events.
	LocalTimeline history.Timeline

	sync.Mutex
	health.Checkers

	metricsListener net.Listener
	// debugListener is the unix domain socket listener for the debug endpoint
	debugListener net.Listener

	// RPC server used by agent for client communication as well as
	// status sync with other agents.
	rpc RPCServer

	// dialRPC is a factory function to create clients to other agents.
	dialRPC client.DialRPC

	// localStatus is the last obtained local node status.
	localStatus *pb.NodeStatus

	// statusQueryReplyTimeout specifies the maximum amount of time to wait for status reply
	// from remote nodes during status collection.
	// Defaults to statusQueryReplyTimeout if unspecified
	statusQueryReplyTimeout time.Duration

	// lastSeen keeps track of the last seen timestamp of an event from a
	// specific cluster member.
	// The last seen timestamp can be queried by a member and be used to
	// filter out events that have already been recorded by this agent.
	lastSeen *ttlmap.TTLMap

	// cancel is used to cancel the internal processes
	// running as part of g
	cancel context.CancelFunc
	// g manages the internal agent's processes
	g ctxgroup.Group
}

// New creates an instance of an agent based on configuration options given in config.
func New(config *Config) (*agent, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	metricsListener, err := net.Listen("tcp", config.MetricsAddr)
	if err != nil {
		return nil, trace.Wrap(err, "failed to bind on %v to serve metrics", config.MetricsAddr)
	}

	var debugListener net.Listener
	if config.DebugSocketPath != "" {
		if err := os.Remove(config.DebugSocketPath); err != nil && !os.IsNotExist(err) {
			return nil, trace.ConvertSystemError(err)
		}
		debugListener, err = net.Listen("unix", config.DebugSocketPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	localTimeline, err := initTimeline(config.TimelineConfig, "local.db")
	if err != nil {
		return nil, trace.Wrap(err, "failed to initialize local timeline")
	}

	// Only initialize cluster timeline for master nodes.
	var (
		clusterTimeline history.Timeline
		lastSeen        *ttlmap.TTLMap
	)
	if role, ok := config.Tags["role"]; ok && Role(role) == RoleMaster {
		clusterTimeline, err = initTimeline(config.TimelineConfig, "cluster.db")
		if err != nil {
			return nil, trace.Wrap(err, "failed to initialize timeline")
		}

		lastSeen = ttlmap.NewTTLMap(lastSeenCapacity)
	}

	ctx, cancel := context.WithCancel(context.Background())
	g := ctxgroup.WithContext(ctx)

	agent := &agent{
		Config:                  *config,
		ClusterTimeline:         clusterTimeline,
		LocalTimeline:           localTimeline,
		dialRPC:                 config.DialRPC,
		statusQueryReplyTimeout: statusQueryReplyTimeout,
		localStatus:             emptyNodeStatus(config.Name),
		metricsListener:         metricsListener,
		debugListener:           debugListener,
		lastSeen:                lastSeen,
		cancel:                  cancel,
		g:                       g,
	}

	agent.rpc, err = newRPCServer(agent, config.CAFile, config.CertFile, config.KeyFile, config.RPCAddrs)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create RPC server")
	}
	return agent, nil
}

// initTimeline initializes a new sqlite timeline. fileName specifies the
// SQLite database file name.
func initTimeline(config sqlite.Config, fileName string) (history.Timeline, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timelineInitTimeout)
	defer cancel()
	config.DBPath = path.Join(config.DBPath, fileName)
	return sqlite.NewTimeline(ctx, config)
}

// GetConfig returns the agent configuration.
func (r *agent) GetConfig() Config {
	return r.Config
}

// Run starts the agent and blocks until it finishes.
// Returns the error (if any) the agent exited with
func (r *agent) Run() (err error) {
	r.Start()
	return trace.Wrap(r.g.Wait())
}

// Start starts the agent's background tasks.
func (r *agent) Start() error {
	r.g.GoCtx(r.recycleLoop)
	r.g.GoCtx(r.statusUpdateLoop)
	return nil
}

// Close stops all background activity and releases the agent's resources.
func (r *agent) Close() (err error) {
	r.rpc.Stop()
	r.cancel()
	var errors []error
	if err := r.g.Wait(); err != nil && !utils.IsContextCanceledError(err) {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// Time reports the current server time.
func (r *agent) Time() time.Time {
	return r.Clock.Now()
}

// LocalStatus reports the status of the local agent node.
func (r *agent) LocalStatus() *pb.NodeStatus {
	return r.recentLocalStatus()
}

// LastSeen returns the last seen timestamp from the specified member.
// If no value is stored for the specific member, a timestamp will be
// initialized for the member with the zero value.
func (r *agent) LastSeen(name string) (lastSeen time.Time, err error) {
	if !hasRoleMaster(r.Tags) {
		return lastSeen, trace.BadParameter("requesting last seen timestamp from non master")
	}

	r.Lock()
	defer r.Unlock()

	if val, ok := r.lastSeen.Get(name); ok {
		if lastSeen, ok = val.(time.Time); !ok {
			return lastSeen, trace.BadParameter("got invalid type %T", val)
		}
	}

	// Reset ttl if successfully retrieved lastSeen.
	// Initialize value if lastSeen had not been previously stored.
	if err := r.lastSeen.Set(name, time.Time{}, lastSeenTTLSeconds); err != nil {
		return lastSeen, trace.Wrap(err, fmt.Sprintf("failed to initialize timestamp for %s", name))
	}

	return lastSeen, nil
}

// RecordLastSeen records the timestamp for the specified member.
// Attempts to record a last seen timestamp that is older than the currently
// recorded timestamp will be ignored.
func (r *agent) RecordLastSeen(name string, timestamp time.Time) error {
	if !hasRoleMaster(r.Tags) {
		return trace.BadParameter("attempting to record last seen timestamp for non master")
	}

	r.Lock()
	defer r.Unlock()

	var lastSeen time.Time
	if val, ok := r.lastSeen.Get(name); ok {
		if lastSeen, ok = val.(time.Time); !ok {
			return trace.BadParameter("got invalid type %T", val)
		}
	}

	// Ignore timestamp that is older than currently stored last seen timestamp.
	if timestamp.Before(lastSeen) {
		return nil
	}

	return r.lastSeen.Set(name, timestamp, lastSeenTTLSeconds)
}

// runChecks executes the monitoring tests configured for this agent in parallel.
func (r *agent) runChecks(ctx context.Context) *pb.NodeStatus {
	// semaphoreCh limits the number of concurrent checkers
	semaphoreCh := make(chan struct{}, maxConcurrentCheckers)
	// channel for collecting resulting health probes
	probeCh := make(chan health.Probes, len(r.Checkers))

	ctxChecks, cancelChecks := context.WithTimeout(ctx, checksTimeout)
	defer cancelChecks()

	for _, c := range r.Checkers {
		select {
		case semaphoreCh <- struct{}{}:
			go runChecker(ctxChecks, c, probeCh, semaphoreCh)
		case <-ctx.Done():
			log.Warnf("Timed out running tests: %v.", ctx.Err())
			return emptyNodeStatus(r.Name)
		}
	}

	var probes health.Probes
	for i := 0; i < len(r.Checkers); i++ {
		select {
		case probe := <-probeCh:
			probes = append(probes, probe...)
		case <-ctx.Done():
			log.Warnf("Timed out collecting test results: %v.", ctx.Err())
			return &pb.NodeStatus{
				Name:   r.Name,
				Status: pb.NodeStatus_Degraded,
				Probes: probes.GetProbes(),
			}
		}
	}

	return &pb.NodeStatus{
		Name:   r.Name,
		Status: probes.Status(),
		Probes: probes.GetProbes(),
	}
}

// GetTimeline returns the current cluster timeline.
func (r *agent) GetTimeline(ctx context.Context, params map[string]string) ([]*pb.TimelineEvent, error) {
	if hasRoleMaster(r.Tags) {
		return r.ClusterTimeline.GetEvents(ctx, params)
	}
	return nil, trace.BadParameter("requesting cluster timeline from non master")
}

// RecordClusterEvents records the events into the cluster timeline.
// Cluster timeline can only be updated if agent has role 'master'.
func (r *agent) RecordClusterEvents(ctx context.Context, events []*pb.TimelineEvent) error {
	if hasRoleMaster(r.Tags) {
		return r.ClusterTimeline.RecordEvents(ctx, events)
	}
	return trace.BadParameter("attempting to update cluster timeline of non master")
}

// RecordLocalEvents records the events into the local timeline.
func (r *agent) RecordLocalEvents(ctx context.Context, events []*pb.TimelineEvent) error {
	return r.LocalTimeline.RecordEvents(ctx, events)
}

// runChecker executes the specified checker and reports results on probeCh.
// If the checker panics, the resulting probe will describe the checker failure.
// Semaphore channel is guaranteed to receive a value upon completion.
func runChecker(ctx context.Context, checker health.Checker, probeCh chan<- health.Probes, semaphoreCh <-chan struct{}) {
	defer func() {
		if err := recover(); err != nil {
			var probes health.Probes
			probes.Add(&pb.Probe{
				Checker:  checker.Name(),
				Status:   pb.Probe_Failed,
				Severity: pb.Probe_Critical,
				Error:    trace.Errorf("checker panicked: %v\n%s", err, debug.Stack()).Error(),
			})
			probeCh <- probes
		}
		// release checker slot
		<-semaphoreCh
	}()

	log.Debugf("Running checker %q.", checker.Name())

	ctxProbe, cancelProbe := context.WithTimeout(ctx, probeTimeout)
	defer cancelProbe()

	checkCh := make(chan health.Probes, 1)
	go func() {
		started := time.Now()

		var probes health.Probes
		checker.Check(ctxProbe, &probes)
		checkCh <- probes

		log.Debugf("Checker %q completed in %v", checker.Name(), time.Since(started))
	}()

	select {
	case probes := <-checkCh:
		probeCh <- probes
	case <-ctx.Done():
		var probes health.Probes
		probes.Add(&pb.Probe{
			Checker:  checker.Name(),
			Status:   pb.Probe_Failed,
			Severity: pb.Probe_Critical,
			Error:    "checker does not comply with specified context, potential goroutine leak",
		})
		probeCh <- probes
	}
}

// recycleLoop periodically recycles the cache.
func (r *agent) recycleLoop(ctx context.Context) error {
	ticker := r.Clock.NewTicker(recycleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			if err := r.Cache.Recycle(); err != nil {
				log.WithError(err).Warn("Error recycling status.")
			}

		case <-ctx.Done():
			log.Info("Recycle loop is stopping.")
			return nil
		}
	}
}

// statusUpdateLoop is a long running background process that periodically
// updates the health status of the cluster by querying status of other active
// cluster members.
func (r *agent) statusUpdateLoop(ctx context.Context) error {
	ticker := r.Clock.NewTicker(StatusUpdateTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			if err := r.updateStatus(ctx); err != nil {
				log.WithError(err).Warn("Failed to updates status.")
			}

		case <-ctx.Done():
			log.Info("Status update loop is stopping.")
			return nil
		}
	}
}

// updateStatus updates the current status.
func (r *agent) updateStatus(ctx context.Context) error {
	ctxStatus, cancel := context.WithTimeout(ctx, r.statusQueryReplyTimeout)
	defer cancel()
	status := r.collectStatus(ctxStatus)
	if err := r.Cache.UpdateStatus(status); err != nil {
		return trace.Wrap(err, "error updating system status in cache")
	}
	return nil
}

func (r *agent) defaultUnknownStatus() *pb.NodeStatus {
	return &pb.NodeStatus{
		Name: r.Name,
		MemberStatus: &pb.MemberStatus{
			Name: r.Name,
		},
	}
}

// collectStatus obtains the cluster status by querying statuses of
// known cluster members.
func (r *agent) collectStatus(ctx context.Context) *pb.SystemStatus {
	ctx, cancel := context.WithTimeout(ctx, StatusUpdateTimeout)
	defer cancel()

	members, err := r.Cluster.Members()
	if err != nil {
		log.WithError(err).Error("Failed to query cluster members.")
		r.setLocalStatus(r.defaultUnknownStatus())
		return &pb.SystemStatus{
			Status:    pb.SystemStatus_Degraded,
			Timestamp: pb.NewTimeToProto(r.Clock.Now()),
			Summary:   fmt.Sprintf("failed to query cluster members: %v", err),
		}
	}

	systemStatus := &pb.SystemStatus{
		Status:    pb.SystemStatus_Unknown,
		Timestamp: pb.NewTimeToProto(r.Clock.Now()),
	}

	log.Debugf("Started collecting statuses from members %v.", members)

	statusCh := make(chan *statusResponse, len(members))
	for _, member := range members {
		if r.Name == member.Name {
			go func() {
				ctxNode, cancelNode := context.WithTimeout(ctx, nodeStatusTimeoutLocal)
				defer cancelNode()

				r.getLocalStatus(ctxNode, statusCh)
			}()
		} else {
			go func(member *pb.MemberStatus) {
				ctxNode, cancelNode := context.WithTimeout(ctx, nodeStatusTimeoutRemote)
				defer cancelNode()

				r.getStatusFrom(ctxNode, member, statusCh)
			}(member)
		}
	}

L:
	for i := 0; i < len(members); i++ {
		select {
		case status := <-statusCh:
			log.Debugf("Retrieved status from %v: %v.", status.member, status.NodeStatus)
			nodeStatus := status.NodeStatus
			if status.err != nil {
				log.Debugf("Failed to query node %s(%v) status: %v.",
					status.member.Name, status.member.Addr, status.err)
				nodeStatus = unknownNodeStatus(status.member)
			}
			systemStatus.Nodes = append(systemStatus.Nodes, nodeStatus)
		case <-ctx.Done():
			log.Warnf("Timed out collecting node statuses: %v.", ctx.Err())
			// With insufficient status responses received, system status
			// will be automatically degraded
			break L
		}
	}

	setSystemStatus(systemStatus, members)

	go r.clientCache.CloseMissingMembers(members)

	return systemStatus
}

// collectLocalStatus executes monitoring tests on the local node.
func (r *agent) collectLocalStatus(ctx context.Context) (status *pb.NodeStatus, err error) {
	local, err := r.Cluster.Member(r.Name)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query local cluster member")
	}

	status = r.runChecks(ctx)
	status.MemberStatus = local

	r.Lock()
	changes := history.DiffNode(r.Clock, r.localStatus, status)
	r.localStatus = status
	r.Unlock()

	// TODO: handle recording of timeline outside of collection.
	if err := r.LocalTimeline.RecordEvents(ctx, changes); err != nil {
		return status, trace.Wrap(err, "failed to record local timeline events")
	}

	if err := r.notifyMasters(ctx); err != nil {
		return status, trace.Wrap(err, "failed to notify master nodes of local timeline events")
	}

	return status, nil
}

// getLocalStatus obtains local node status.
func (r *agent) getLocalStatus(ctx context.Context, respc chan<- *statusResponse) {
	// TODO: restructure code so that local member is not needed here.
	local, err := r.Cluster.Member(r.Name)
	if err != nil {
		respc <- &statusResponse{err: err}
		return
	}
	status, err := r.collectLocalStatus(ctx)
	resp := &statusResponse{
		NodeStatus: status,
		member:     local,
		err:        err,
	}
	select {
	case respc <- resp:
	case <-ctx.Done():
	}
}

// notifyMasters pushes new timeline events to all master nodes in the cluster.
func (r *agent) notifyMasters(ctx context.Context) error {
	members, err := r.Cluster.Members()
	if err != nil {
		return trace.Wrap(err)
	}

	events, err := r.LocalTimeline.GetEvents(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: async
	for _, member := range members {
		if !hasRoleMaster(member.Tags) {
			continue
		}
		if err := r.notifyMaster(ctx, member, events); err != nil {
			log.WithError(err).Debugf("Failed to notify %s of new timeline events.", member.Name)
		}
	}

	return nil
}

// notifyMaster push new timeline events to the specified member.
func (r *agent) notifyMaster(ctx context.Context, member *pb.MemberStatus, events []*pb.TimelineEvent) error {
	client, err := r.DialRPC(ctx, member.Addr)
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := client.LastSeen(ctx, &pb.LastSeenRequest{Name: r.Name})
	if err != nil {
		return trace.Wrap(err)
	}

	// Filter out previously recorded events.
	filtered := filterByTimestamp(events, resp.GetTimestamp().ToTime())

	for _, event := range filtered {
		if _, err := client.UpdateTimeline(ctx, &pb.UpdateRequest{Name: r.Name, Event: event}); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// getStatusFrom obtains node status from the node identified by member.
func (r *agent) getStatusFrom(ctx context.Context, member *pb.MemberStatus, respc chan<- *statusResponse) {
	client, err := r.DialRPC(ctx, member.Addr)
	resp := &statusResponse{member: member}
	if err != nil {
		resp.err = trace.Wrap(err)
	} else {
		var status *pb.NodeStatus
		status, err = client.LocalStatus(ctx)
		if err != nil {
			resp.err = trace.Wrap(err)
		} else {
			resp.NodeStatus = status
		}
	}
	select {
	case respc <- resp:
	case <-ctx.Done():
	}
}

// Status returns the last known cluster status.
func (r *agent) Status() (status *pb.SystemStatus, err error) {
	status, err = r.Cache.RecentStatus()
	if err == nil && status == nil {
		status = pb.EmptyStatus()
	}
	return status, trace.Wrap(err)
}

// recentLocalStatus returns the last known local node status.
func (r *agent) recentLocalStatus() *pb.NodeStatus {
	r.Lock()
	defer r.Unlock()
	return r.localStatus
}

func (r *agent) setLocalStatus(status *pb.NodeStatus) {
	r.Lock()
	defer r.Unlock()
	r.localStatus = status
}

// filterByTimestamp filters out events that occurred before the provided
// timestamp.
func filterByTimestamp(events []*pb.TimelineEvent, timestamp time.Time) (filtered []*pb.TimelineEvent) {
	for _, event := range events {
		if event.GetTimestamp().ToTime().Before(timestamp) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

// hasRoleMaster returns true if tags contains role 'master'.
func hasRoleMaster(tags map[string]string) bool {
	role, ok := tags["role"]
	return ok && Role(role) == RoleMaster
}

// statusResponse describes a status response from a background process that obtains
// health status on the specified node.
type statusResponse struct {
	*pb.NodeStatus
	member *pb.MemberStatus
	err    error
}
