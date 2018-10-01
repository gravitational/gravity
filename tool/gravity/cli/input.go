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

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"

	"github.com/fatih/color"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
)

func readCheck(prompt string, fn func(v string) (string, error)) (string, error) {
	for {
		out, err := readInput(prompt)
		if err != nil {
			return "", err
		}
		out, err = fn(out)
		if err != nil {
			continue
		}
		return out, nil
	}
}

func notEmpty(v string) (string, error) {
	if v == "" {
		return "", fmt.Errorf("value can not be empty")
	}
	return v, nil
}

func validDomain(v string) (string, error) {
	if !cstrings.IsValidDomainName(v) {
		return "", fmt.Errorf("value should be a valid domain")
	}
	return v, nil
}

// selectInterface returns IP address of the network interface selected
// for the installation
//
// If the machine has a single network interface (not counting loopback),
// it is returned right away. Otherwise, a user is shown a prompt dialog
// where they can pick an interface.
func selectInterface() (addr string, autoselected bool, err error) {
	ifaces, err := systeminfo.NetworkInterfaces()
	if err != nil {
		return "", false, trace.Wrap(err)
	}
	if len(ifaces) == 0 {
		return "", false, trace.Errorf("no network interfaces found")
	}
	if len(ifaces) == 1 {
		return ifaces[0].IPv4, true, nil
	}
	fmt.Printf("\nSelect an interface for the installer to listen on:\n\n")

	num2iface := make(map[string]storage.NetworkInterface)
	number := 0
	for _, iface := range ifaces {
		if iface.IPv4 != "" && iface.IPv4 != "<nil>" {
			number += 1
			num2iface[fmt.Sprintf("%v", number)] = iface
			fmt.Printf("%v. %v\n", number, iface.IPv4)
		}
	}
	fmt.Printf(color.YellowString("\nNote: Target servers should be able to connect to this IP\n"))

	addr, err = readCheck(fmt.Sprintf("\nSelect interface number [%v-%v]", 1, number), func(number string) (string, error) {
		iface, ok := num2iface[number]
		if !ok {
			return "", fmt.Errorf("select interface number")
		}
		return iface.IPv4, nil
	})
	if err != nil {
		return "", false, trace.Wrap(err)
	}

	return addr, false, nil
}

func checkYesNo(v string) (string, error) {
	switch v {
	case "Y", "y", "yes":
		return "true", nil
	case "N", "n", "no":
		return "false", nil
	}
	return "", trace.BadParameter("invalid input: %v", v)
}

func confirm() (bool, error) {
	return confirmWithTitle("confirm")
}

func confirmWithTitle(title string) (bool, error) {
	input, err := readCheck(fmt.Sprintf("%v (yes/no)", title), checkYesNo)
	if err != nil {
		return false, trace.Wrap(err)
	}
	b, err := strconv.ParseBool(input)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return b, nil
}

func enforceConfirmation(title string, args ...interface{}) error {
	confirmed, err := confirmWithTitle(fmt.Sprintf(title, args...))
	if err != nil {
		return trace.Wrap(err)
	}
	if !confirmed {
		return trace.CompareFailed("cancelled")
	}
	return nil
}

func readInput(prompt string) (string, error) {
	fmt.Fprintf(os.Stdout, "%v:\n", prompt)
	reader := bufio.NewReader(os.Stdin)
	bytes, err := reader.ReadSlice('\n')
	if err != nil {
		return "", trace.Wrap(err)
	}
	return strings.TrimSpace(string(bytes)), nil
}
