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

package report

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"
)

// NewSystemCollector returns a list of collectors to fetch system information
func NewSystemCollector() Collectors {
	var collectors Collectors
	add := func(additional ...Collector) {
		collectors = append(collectors, additional...)
	}

	add(basicSystemInfo()...)
	add(planetServices()...)
	add(syslogExportLogs())
	add(systemFileLogs()...)
	add(planetLogs()...)
	add(bashHistoryCollector{})

	return collectors
}

func basicSystemInfo() Collectors {
	return Collectors{
		// networking
		Cmd("iptables", "iptables-save"),
		Cmd("route", "route", "-n"),
		Cmd("ifconfig", "ifconfig", "-a"),
		Cmd("ip-addr", "ip", "-d", "addr", "show"),
		Cmd("ip-route", "ip", "-d", "route", "show", "table", "all"),
		Cmd("ip-link", "ip", "-d", "link", "show"),
		Cmd("ip-neighbor", "ip", "-d", "neighbor", "show"),
		Cmd("ip-rule", "ip", "-d", "rule", "show"),
		Cmd("ip-ntable", "ip", "-d", "ntable", "show"),
		Cmd("ip-maddress", "ip", "-d", "maddress", "show"),
		Cmd("ip-xfrm-state", "ip", "-d", "xfrm", "state", "show"),
		Cmd("ip-xfrm-policy", "ip", "-d", "xfrm", "policy", "show"),
		Cmd("ip-tcp_metrics", "ip", "-d", "tcp_metrics", "show"),
		Cmd("ip-netconf", "ip", "-d", "netconf", "show"),
		Cmd("bridge-fdb", utils.PlanetCommandArgs("/sbin/bridge", "fdb", "show")...),
		// disk
		Cmd("lsblk", "lsblk"),
		Cmd("fdisk", "fdisk", "-l"),
		Cmd("dmsetup", "dmsetup", "info"),
		Cmd("df", "df", "-hT"),
		Cmd("df-inodes", "df", "-i"),
		// system
		Cmd("lscpu", "lscpu"),
		Cmd("lsmod", "lsmod"),
		Cmd("running-processes", "/bin/ps", "aux", "--forest"),
		Cmd("systemctl-host", "/bin/systemctl", "status"),
		Cmd("dmesg", "cat", "/var/log/dmesg"),
		// Fetch world-readable parts of /etc/
		Script("etc-logs.tar.gz", tarball("/etc/")),
		// memory
		Cmd("free", "free", "--human"),
		Cmd("slabtop", "slabtop", "--once"),
		Cmd("vmstat", "vmstat", "--stats"),
		Cmd("slabinfo", "cat", "/proc/slabinfo"),
	}
}

func planetServices() Collectors {
	return Collectors{
		// etcd cluster health
		Cmd("etcdctl", utils.PlanetCommandArgs("/usr/bin/etcdctl", "cluster-health")...),
		Cmd("planet-status", utils.PlanetCommandArgs("/usr/bin/planet", "status")...),
		// status of systemd units
		Cmd("systemctl", utils.PlanetCommandArgs("/bin/systemctl", "status")...),
	}
}

// syslogExportLogs fetches logs for gravity binary invocations
// (including installation logs)
func syslogExportLogs() Collector {
	const script = `
#!/bin/bash
/bin/journalctl --no-pager --output=export %v | /bin/gzip -f`
	syslogID := func(id string) string {
		return fmt.Sprintf("SYSLOG_IDENTIFIER=%v", id)
	}
	matches := []string{
		syslogID("./gravity"),
		syslogID("gravity"),
		syslogID(defaults.GravityBin),
	}

	return Script("gravity-system.log.gz", fmt.Sprintf(script, strings.Join(matches, " ")))
}

// systemFileLogs fetches gravity platform-related logs
func systemFileLogs() Collectors {
	const template = `
#!/bin/bash
cat %v 2> /dev/null || true`
	return Collectors{
		Script(filepath.Base(defaults.GravitySystemLog), fmt.Sprintf(template, defaults.GravitySystemLog)),
		Script(filepath.Base(defaults.GravityUserLog), fmt.Sprintf(template, defaults.GravityUserLog)),
	}
}

// planetLogs fetches planet syslog messages as well as the fresh journal entries
func planetLogs() Collectors {
	return Collectors{
		// Fetch planet syslog messages as a tarball
		Script("planet-logs.tar.gz", tarball(defaults.InGravity("planet/log/messages*"))),
		// Fetch planet journal entries for the last two days
		// The log can be imported as a journal with systemd-journal-remote:
		//
		// $ cat ./node-1-planet-journal-export.log | /lib/systemd/systemd-journal-remote -o ./journal/system.journal -
		Self("planet-journal-export.log.gz",
			"system", "export-runtime-journal"),
	}
}

// etcdBackup fetches etcd data for gravity and planet
func etcdBackup() Collectors {
	return Collectors{
		Cmd("etcd-backup.json", utils.PlanetCommandArgs(defaults.PlanetBin, "etcd", "backup",
			"--prefix", defaults.EtcdPlanetPrefix,
			"--prefix", defaults.EtcdGravityPrefix)...),
	}
}
