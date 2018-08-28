package ebtables

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

const (
	cmd = "/sbin/ebtables"

	// Flag to show full mac in output. The default representation omits leading zeroes.
	fullMac = "--Lmac2"
)

// RulePosition describes the position of a rule in the chain
type RulePosition string

const (
	// Prepend defines a rule position to place a new rule at the head of the chain
	Prepend RulePosition = "-I"
	// Append defines a rule position to place a new rule at the end of the chain
	Append RulePosition = "-A"
)

// Table is an ebtables table
type Table string

const (
	// TableFilter identifies the filter table
	TableFilter Table = "filter"
)

// Chain is an ebtables chain
type Chain string

const (
	// ChainPostrouting identifies the POSTROUTING chain
	ChainPostrouting Chain = "POSTROUTING"
	// ChainPrerouting identifies the PREROUTING chain
	ChainPrerouting Chain = "PREROUTING"
	// ChainOutput identifies the OUTPUT chain
	ChainOutput Chain = "OUTPUT"
	// ChainInput identifies the INPUT chain
	ChainInput Chain = "INPUT"
)

type operation string

const (
	opCreateChain operation = "-N"
	opFlushChain  operation = "-F"
	opDeleteChain operation = "-X"
	opListChain   operation = "-L"
	opAppendRule  operation = "-A"
	opPrependRule operation = "-I"
	opDeleteRule  operation = "-D"
)

// GetVersion returns the "X.Y.Z" semver string for ebtables
func GetVersion() (string, error) {
	// this doesn't access mutable state so we don't need to use the interface / runner
	out, err := run(cmd, "--version")
	if err != nil {
		return "", err
	}
	versionMatcher := regexp.MustCompile("v([0-9]+\\.[0-9]+\\.[0-9]+)")
	match := versionMatcher.FindStringSubmatch(string(out))
	if len(match) == 0 {
		return "", trace.NotFound("no ebtables version found in string %s", out)
	}
	return match[1], nil
}

// EnsureRule checks if the specified rule is present and, if not, creates it.
// WARNING: ebtables does not provide check operation like iptables do. Hence we have to do a string match of args.
// Input args must follow the format and sequence of ebtables list output. Otherwise, EnsureRule will always create
// new rules and cause duplicates.
func EnsureRule(position RulePosition, table Table, chain Chain, args ...string) error {
	var exists bool
	fullArgs := makeFullArgs(table, opListChain, chain, fullMac)
	out, err := run(cmd, fullArgs...)
	if err == nil {
		exists = checkIfRuleExists(string(out), args...)
	}
	if !exists {
		fullArgs = makeFullArgs(table, operation(position), chain, args...)
		out, err := run(cmd, fullArgs...)
		if err != nil {
			return trace.Wrap(err, "failed to ensure rule: %s", out)
		}
	}
	return nil
}

// EnsureChain checks if the specified chain is present and, if not, creates it
func EnsureChain(table Table, chain Chain) error {
	exists := true
	args := makeFullArgs(table, opListChain, chain)
	_, err := run(cmd, args...)
	if err != nil {
		exists = false
	}

	if !exists {
		args = makeFullArgs(table, opCreateChain, chain)
		out, err := run(cmd, args...)
		if err != nil {
			return trace.Wrap(err, "failed to ensure %q chain: %s", chain, out)
		}
	}
	return nil
}

// FlushChain flushes the specified chain. Returns error if the chain does not exist.
func FlushChain(table Table, chain Chain) error {
	fullArgs := makeFullArgs(table, opFlushChain, chain)
	out, err := run(cmd, fullArgs...)
	if err != nil {
		return trace.Wrap(err, "failed to flush %q chain %q: %s", table, chain, out)
	}
	return nil
}

// DeleteRule deletes the specified rule. Returns error if the chain does not exist.
func DeleteRule(table Table, chain Chain, args ...string) error {
	fullArgs := makeFullArgs(table, opDeleteRule, chain, args...)
	out, err := run(cmd, fullArgs...)
	if err != nil {
		return trace.Wrap(err, "failed to delete %v chain %v rule %v: %s", table, chain, args, out)
	}
	return nil
}

// DeleteChain deletes the specified chain. Returns error if the chain does not exist.
func DeleteChain(table Table, chain Chain) error {
	fullArgs := makeFullArgs(table, opDeleteChain, chain)
	out, err := run(cmd, fullArgs...)
	if err != nil {
		return trace.Wrap(err, "failed to delete %v chain %v: %s", table, chain, out)
	}
	return nil
}

// checkIfRuleExists takes the output of 'ebtables list chain' and checks if the input rule exists
func checkIfRuleExists(listChainOutput string, args ...string) bool {
	rule := strings.Join(args, " ")
	for _, line := range strings.Split(listChainOutput, "\n") {
		if strings.TrimSpace(line) == rule {
			return true
		}
	}
	return false
}

func makeFullArgs(table Table, op operation, chain Chain, args ...string) []string {
	return append([]string{"-t", string(table), string(op), string(chain)}, args...)
}

func run(cmd string, args ...string) (output []byte, err error) {
	var out bytes.Buffer
	err = utils.Exec(exec.Command(cmd, args...), &out)
	return out.Bytes(), trace.ConvertSystemError(err)
}
