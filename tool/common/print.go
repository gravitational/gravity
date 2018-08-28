package common

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
)

// PrintError prints the red error message to the console
func PrintError(err error) {
	color.Red("[ERROR]: %v\n", trace.UserMessage(err))
}

// PrintHeader formats the provided string as a header and prints it to the console
func PrintHeader(val string) {
	fmt.Printf("\n[%v]\n%v\n", val, strings.Repeat("-", len(val)+2))
}

// PrintTableHeader prints header of a table
func PrintTableHeader(w io.Writer, cols []string) {
	dots := make([]string, len(cols))
	for i := range dots {
		dots[i] = strings.Repeat("-", len(cols[i]))
	}
	fmt.Fprint(w, strings.Join(cols, "\t")+"\n")
	fmt.Fprint(w, strings.Join(dots, "\t")+"\n")
}
