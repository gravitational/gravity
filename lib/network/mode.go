package network

import (
	"bytes"
	"net"
	"os/exec"

	tool "github.com/gravitational/gravity/lib/network/ebtables"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// SetPromiscuousMode puts the specified interface iface into promiscuous mode
// and configures ebtable rules to eliminate duplicate packets.
func SetPromiscuousMode(ifaceName string) error {
	log.Debugf("set promiscuous mode on %q", ifaceName)
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return trace.Wrap(err)
	}

	addrs, err := getAddrs(*iface)
	if err != nil {
		return trace.Wrap(err)
	}

	var out bytes.Buffer
	err = utils.Exec(exec.Command(cmdIP, "link", "show", "dev", ifaceName), &out)
	if err != nil {
		return trace.Wrap(err)
	}
	if !bytes.Contains(out.Bytes(), promiscuousModeOn) {
		out.Reset()
		err = utils.Exec(exec.Command(cmdIP, "link", "set", ifaceName, "promisc", "on"), &out)
		if err != nil {
			return trace.Wrap(err, "error setting promiscuous mode on %q: %v", ifaceName, &out)
		}
	}

	var errors []error
	for _, addr := range addrs {
		// configure the ebtables rules to eliminate duplicate packets by best effort
		err := syncEbtablesDedupRules(iface.HardwareAddr, ifaceName, addr.ip.String(), addr.network.String())
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) != 0 {
		return trace.NewAggregate(errors...)
	}

	return nil
}

// UnsetPromiscuousMode removes the promiscuous mode flag and deletes the deduplication
// chain set up for the specified interface ifaceName
func UnsetPromiscuousMode(ifaceName string) error {
	var out bytes.Buffer
	err := utils.Exec(exec.Command(cmdIP, "link", "set", ifaceName, "promisc", "off"), &out)
	if err != nil {
		return trace.Wrap(err, "failed to unset promiscuous mode on %q: %v", ifaceName, &out)
	}

	err = tool.DeleteRule(tool.TableFilter, tool.ChainOutput, "-j", string(dedupChain))
	if err != nil {
		log.Warnf("failed to delete jump rule: %v", trace.DebugReport(err))
	}

	return trace.Wrap(tool.DeleteChain(tool.TableFilter, dedupChain))
}

func syncEbtablesDedupRules(macAddr net.HardwareAddr, ifaceName string, gateway, network string) error {
	if err := tool.FlushChain(tool.TableFilter, dedupChain); err != nil {
		log.Debugf("failed to flush deduplication chain: %v", err)
	}

	_, err := tool.GetVersion()
	if err != nil {
		return trace.Wrap(err, "failed to get ebtables version")
	}

	log.Debugf("filtering packets with ebtables on mac address %v, gateway %q, network %q", macAddr, gateway, network)
	err = tool.EnsureChain(tool.TableFilter, dedupChain)
	if err != nil {
		return trace.Wrap(err, "failed to create/update %q chain %q", tool.TableFilter, dedupChain)
	}

	// Jump from OUTPUT chain to deduplication chain in the filter table
	err = tool.EnsureRule(tool.Append, tool.TableFilter, tool.ChainOutput, "-j", string(dedupChain))
	if err != nil {
		return trace.Wrap(err, "failed to ensure %v chain %v rule to jump to %v chain",
			tool.TableFilter, tool.ChainOutput, dedupChain)
	}

	commonArgs := []string{"-p", "IPv4", "-s", macAddr.String(), "-o", "veth+"}
	// Allow the gateway IP address when the source is the specified mac address
	err = tool.EnsureRule(tool.Prepend, tool.TableFilter, dedupChain,
		append(commonArgs, "--ip-src", gateway, "-j", "ACCEPT")...)
	if err != nil {
		return trace.Wrap(err, "failed to ensure rule for packets from %q gateway to be accepted", ifaceName)

	}

	// Block any other IP from pod subnet sourced with the specified mac address
	err = tool.EnsureRule(tool.Append, tool.TableFilter, dedupChain,
		append(commonArgs, "--ip-src", network, "-j", "DROP")...)
	if err != nil {
		return trace.Wrap(err, "failed to ensure rule to drop packets from %v but with mac address of %q",
			network, ifaceName)
	}

	return nil
}

func getAddrs(iface net.Interface) (result []bridgeAddr, err error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, trace.Wrap(err, "failed to list interface addresses")
	}

	for _, addr := range addrs {
		switch ipAddr := addr.(type) {
		case *net.IPNet:
			ip := ipAddr.IP.To4()
			if ip != nil {
				result = append(result, bridgeAddr{ip: ip, network: network(ipAddr)})
			}
		}
	}

	if len(result) != 0 {
		return result, nil
	}

	return nil, trace.NotFound("no addresses found")
}

// network masks off the host portion of the IP address:
//
// given IP 172.17.0.1 and mask 255.255.0.0, returns 172.17.0.0/16
func network(ipNet *net.IPNet) net.IPNet {
	return net.IPNet{
		IP:   ipNet.IP.Mask(ipNet.Mask),
		Mask: ipNet.Mask,
	}
}

type bridgeAddr struct {
	ip      net.IP
	network net.IPNet
}

// ebtables chain to store deduplication rules
var dedupChain = tool.Chain("KUBE-DEDUP")

// promiscuousModeOn specifies the value of the promiscuous mode flag
// in the output of `ip link show dev <name>`
var promiscuousModeOn = []byte("PROMISC")

const cmdIP = "/sbin/ip"
