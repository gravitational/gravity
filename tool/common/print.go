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

package common

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
)

// PrintError prints the red error message to stderr
func PrintError(err error) {
	fmt.Fprint(os.Stderr, color.RedString("[ERROR]: %v\n", trace.UserMessage(err)))
}

// PrintWarn outputs a warning message to stdout
func PrintWarn(message string, args ...interface{}) {
	fmt.Println(color.YellowString("[WARN] "+message, args...))
}

// PrintHeader formats the provided string as a header and prints it to stdout
func PrintHeader(val string) {
	fmt.Printf("\n[%v]\n%v\n", val, strings.Repeat("-", len(val)+2))
}

// PrintTableHeader prints header of a table
func PrintTableHeader(w io.Writer, cols []string) {
	PrintCustomTableHeader(w, cols, "-")
}

// PrintCustomTableHeader outputs headers using split as a separator
func PrintCustomTableHeader(w io.Writer, headers []string, split string) {
	dots := make([]string, len(headers))
	for i := range dots {
		dots[i] = strings.Repeat(split, len(headers[i]))
	}
	fmt.Fprint(w, strings.Join(headers, "\t")+"\n")
	fmt.Fprint(w, strings.Join(dots, "\t")+"\n")
}
