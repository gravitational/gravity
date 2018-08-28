package report

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"
)

// SystemInfo returns a list of collectors to fetch various bits of system information
func SystemInfo() Collectors {
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
		Cmd("ifconfig", "ifconfig", "-a"),
		Cmd("ipaddr", "ip", "addr"),
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
		Script(filepath.Base(defaults.TelekubeSystemLog), fmt.Sprintf(template, defaults.TelekubeSystemLog)),
		Script(filepath.Base(defaults.TelekubeUserLog), fmt.Sprintf(template, defaults.TelekubeUserLog)),
	}
}

// planetLogs fetches planet syslog messages as well as the fresh journal entries
func planetLogs() Collectors {
	const script = `
#!/bin/bash
/bin/journalctl --since=yesterday --output=export -D %v | /bin/gzip -f`
	return Collectors{
		// Fetch planet syslog messages as a tarball
		Script("planet-logs.tar.gz", tarball(defaults.InGravity("planet/log/messages*"))),
		// Fetch planet journal entries for the last two days
		// The log can be imported as a journal with systemd-journal-remote:
		//
		// $ cat ./node-1-planet-journal-export.log | /lib/systemd/systemd-journal-remote -o ./journal/system.journal -
		Script("planet-journal-export.log.gz",
			fmt.Sprintf(script, defaults.InGravity("planet/log/journal"))),
	}
}
