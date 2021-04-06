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
		return trace.CompareFailed("Operation has been canceled by user.")
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
