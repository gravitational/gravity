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

package opsservice

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/report"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func (s *site) getClusterReport(ctx context.Context, since time.Duration) (io.ReadCloser, error) {
	op, err := storage.GetLastOperationForCluster(s.backend(), s.domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.WithField("op", op).Info("Capture debug report for operation.")
	switch {
	case isActiveInstallOperation((ops.SiteOperation)(*op)):
		return s.getClusterInstallReport(ctx, (ops.SiteOperation)(*op), since)
	default:
		return s.getClusterGenericReport(ctx, since)
	}
}

func (s *site) getClusterInstallReport(ctx context.Context, op ops.SiteOperation, since time.Duration) (io.ReadCloser, error) {
	opCtx, err := s.newOperationContext(op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	entry := s.WithFields(log.Fields{constants.FieldSiteDomain: s.domainName})
	servers, err := s.loadProvisionedServers(op.Servers, 0, entry)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var masterServers []remoteServer
	remoteServers := make([]remoteServer, 0, len(opCtx.provisionedServers))
	for _, server := range servers {
		remoteServers = append(remoteServers, server)
		if server.IsMaster() {
			masterServers = append(masterServers, server)
		}
	}

	runner := s.agentRunner(opCtx)
	return s.getReport(ctx, runner, remoteServers, masterServers, since)
}

func (s *site) getClusterGenericReport(ctx context.Context, since time.Duration) (io.ReadCloser, error) {
	const noRetry = 1
	servers, err := s.getTeleportServersWithTimeout(
		nil,
		defaults.TeleportServerQueryTimeout,
		defaults.RetryInterval,
		noRetry,
		queryReturnsAtLeastOneServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportRunner := &teleportRunner{
		FieldLogger:          log.WithField(trace.Component, "teleport-runner"),
		TeleportProxyService: s.teleport(),
		domainName:           s.domainName,
	}

	var masterServers []remoteServer
	remoteServers := make([]remoteServer, 0, len(servers))
	for _, server := range servers {
		teleportServer, err := newTeleportServer(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		role := schema.ServiceRole(teleportServer.Labels[schema.ServiceLabelRole])
		if role == schema.ServiceRoleMaster {
			masterServers = append(masterServers, teleportServer)
		}
		remoteServers = append(remoteServers, teleportServer)
	}

	return s.getReport(ctx, teleportRunner, remoteServers, masterServers, since)
}

func (s *site) getReport(ctx context.Context, runner remoteRunner, servers []remoteServer, masters []remoteServer,
	since time.Duration) (io.ReadCloser, error) {
	dir, err := ioutil.TempDir("", "report")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = runCollectors(ctx, *s, dir)
	if err != nil {
		// Intermediate steps in diagnostics collection are not fatal
		// to collect all possible pieces in best-effort
		log.WithError(err).Warn("Failed to run cluster collectors.")
	}

	if err := collectOperationsLogs(*s, dir); err != nil {
		log.WithError(err).Warn("Failed to collect operation logs.")
	}

	if len(servers) > 0 {
		// Use the first master server to collect kubernetes diagnostics
		var server remoteServer
		if len(masters) > 0 {
			server = masters[0]
		} else {
			server = servers[0]
			log.Warningf("No master servers, collecting Kubernetes diagnostics from %v.", server)
		}
		serverRunner := &serverRunner{server: server, runner: runner}
		reportWriter := getReportWriterForServer(dir, server)
		logger := log.WithField("server", server.Address())
		if err := s.collectKubernetesInfo(reportWriter, serverRunner, since); err != nil {
			logger.WithError(err).Error("Failed to collect Kubernetes info.")
		}
		if err := s.collectEtcdInfoFromMasters(dir, masters, runner); err != nil {
			log.WithError(err).Error("Failed to collect etcd info.")
		}
		if err := s.collectDebugInfoFromServers(ctx, dir, servers, runner, since); err != nil {
			log.WithError(err).Error("Failed to collect diagnostics from some nodes.")
		}
		if err := s.collectStatusTimeline(reportWriter, serverRunner); err != nil {
			logger.WithError(err).Error("Failed to collect status timeline.")
		}
	}

	// use a pipe to avoid allocating a buffer
	reader, writer := io.Pipe()
	gzWriter := gzip.NewWriter(writer)

	// writing w/o a reader will deadlock so write in a goroutine
	go func() {
		err := archive.CompressDirectory(dir, gzWriter)
		gzWriter.Close()
		writer.CloseWithError(err) //nolint:errcheck
	}()

	return &utils.CleanupReadCloser{
		ReadCloser: reader,
		Cleanup: func() {
			os.RemoveAll(dir)
		},
	}, nil
}

// collectDebugInfoFromServers collects diagnostic information from servers
// and stores each piece into a file in directory dir.
// Files are named using the following pattern:
//
//   <server-name>-<resource>
//
func (s *site) collectDebugInfoFromServers(ctx context.Context, dir string, servers []remoteServer, runner remoteRunner,
	since time.Duration) error {
	err := s.executeOnServers(ctx, servers, func(c context.Context, server remoteServer) error {
		log.WithField("server", server.Debug()).Debug("Collect debug info.")
		r := &serverRunner{
			server: server,
			runner: runner,
		}
		reportWriter := getReportWriterForServer(dir, server)
		err := s.collectDebugInfo(reportWriter, r, since)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *site) collectDebugInfo(reportWriter report.FileWriter, runner *serverRunner, since time.Duration) error {
	w, err := reportWriter.NewWriter("debug-logs.tar.gz")
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	var stderr bytes.Buffer
	err = runner.RunStream(w, &stderr, s.gravityCommand("system", "report",
		"--filter", report.FilterSystem,
		"--compressed",
		"--since", since.String())...)
	if err != nil {
		return trace.Wrap(err, "failed to collect diagnostics: %s", stderr.String())
	}
	return nil
}

func (s *site) collectKubernetesInfo(reportWriter report.FileWriter, runner *serverRunner, since time.Duration) error {
	w, err := reportWriter.NewWriter("k8s-logs.tar.gz")
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	var stderr bytes.Buffer
	err = runner.RunStream(w, &stderr, s.gravityCommand("system", "report",
		"--filter", report.FilterKubernetes,
		"--compressed",
		"--since", since.String())...)
	if err != nil {
		return trace.Wrap(err, "failed to collect kubernetes diagnostics: %s", stderr.String())
	}
	return nil
}

func (s *site) collectEtcdInfoFromMasters(dir string, masters []remoteServer, runner remoteRunner) error {
	err := s.executeOnServers(context.TODO(), masters, func(c context.Context, master remoteServer) error {
		log.Debugf("collectEtcdInfo for %v", master)
		r := &serverRunner{
			server: master,
			runner: runner,
		}
		reportWriter := getReportWriterForServer(dir, master)
		err := s.collectEtcdInfo(reportWriter, r)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// collectEtcdInfo collects etcd metrics and captures a snapshot of the data.
func (s *site) collectEtcdInfo(reportWriter report.FileWriter, runner *serverRunner) error {
	w, err := reportWriter.NewWriter("etcd.json.tar.gz")
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()
	var stderr bytes.Buffer
	err = runner.RunStream(w, &stderr, s.gravityCommand("system", "report", fmt.Sprintf(
		"--filter=%v", report.FilterEtcd), "--compressed")...)
	if err != nil {
		return trace.Wrap(err, "failed to collect etcd info: %s", stderr.String())
	}
	return nil
}

func (s *site) collectStatusTimeline(reportWriter report.FileWriter, runner *serverRunner) error {
	w, err := reportWriter.NewWriter("status.tar.gz")
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()
	var stderr bytes.Buffer
	err = runner.RunStream(w, &stderr, s.gravityCommand("system", "report",
		fmt.Sprintf("--filter=%v", report.FilterTimeline), "--compressed")...)
	if err != nil {
		return trace.Wrap(err, "failed to collect status timeline: %s", stderr.String())
	}
	return nil
}

func runCollectors(ctx context.Context, cluster site, dir string) error {
	storageSite, err := cluster.service.cfg.Backend.GetSite(cluster.domainName)
	if err != nil {
		return trace.Wrap(err)
	}

	collectors := []collectorFn{
		collectClusterInfo(*storageSite),
		collectDumpHook,
	}
	reportWriter := report.NewFileWriter(dir)

	// collect information from all collectors
	for _, collector := range collectors {
		err := collector(ctx, reportWriter, cluster)
		if err != nil {
			log.WithError(err).Error("Failed to collect diagnostics.")
		}
	}
	return nil
}

func collectOperationsLogs(site site, dir string) error {
	operations, err := site.service.GetSiteOperations(site.key)
	if err != nil {
		return trace.Wrap(err, "failed to get cluster operations")
	}

	reportWriter := report.NewFileWriter(dir)

	for _, op := range operations {
		operation := ops.SiteOperation(op)
		err = collectOperationLogs(site, operation, reportWriter)
		if err != nil {
			log.Errorf("failed to collect logs for %q: %v", op.Type, trace.DebugReport(err))
		}
	}
	return nil
}

// collectClusterInfo returns JSON-formatted cluster information
func collectClusterInfo(s storage.Site) collectorFn {
	return func(_ context.Context, reportWriter report.FileWriter, _ site) error {
		w, err := reportWriter.NewWriter(clusterInfoFilename)
		if err != nil {
			return trace.Wrap(err)
		}
		defer w.Close()

		// do not leak license in cluster debug report
		if s.License != "" {
			s.License = "redacted"
		}
		enc := json.NewEncoder(w)
		err = enc.Encode(s)
		return trace.Wrap(err)
	}
}

// collectDumpHook returns the output of the dump hook
func collectDumpHook(ctx context.Context, reportWriter report.FileWriter, site site) error {
	if !site.app.Manifest.HasHook(schema.HookDump) {
		return nil
	}

	w, err := reportWriter.NewWriter(dumpHookFilename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	_, out, err := app.RunAppHook(ctx, site.appService, app.HookRunRequest{
		Application: site.app.Package,
		Hook:        schema.HookDump,
		ServiceUser: site.serviceUser(),
	})
	if err != nil {
		return trace.Wrap(err, string(out))
	}

	_, err = io.Copy(w, bytes.NewReader(out))
	return trace.Wrap(err)
}

// collectOperationLogs streams logs of the specified operation using the specified writer
func collectOperationLogs(site site, operation ops.SiteOperation, reportWriter report.FileWriter) error {
	w, err := reportWriter.NewWriter(fmt.Sprintf(opLogsFilename, operation.Type, operation.ID))
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	f, err := os.Open(site.operationLogPath(operation.Key()))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	return trace.Wrap(err)
}

type collectorFn func(context.Context, report.FileWriter, site) error

func getReportWriterForServer(dir string, server remoteServer) report.FileWriter {
	return report.FileWriterFunc(func(name string) (io.WriteCloser, error) {
		fileName := filepath.Join(dir, fmt.Sprintf("%s-%s", server.HostName(), name))
		return report.NewPendingFileWriter(fileName), nil
	})
}

func isActiveInstallOperation(op ops.SiteOperation) bool {
	return op.Type == ops.OperationInstall && !op.IsCompleted()
}

const (
	// clusterInfoFilename is the name of the file with JSON-encoded cluster metadata
	clusterInfoFilename = "cluster.json"
	// dumpHookFilename is the name of the file with dump hook output
	dumpHookFilename = "dump-hook"
	// opLogsFilename defines the file pattern that stores operation log for a particular
	// cluster operation
	opLogsFilename = "%v.%v"
)
