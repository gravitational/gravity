package common

import (
	"fmt"
	"os"

	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/dustin/go-humanize"
)

// ProgressReporter returns new progress reporter either silent
// or verbose based on the settings
func ProgressReporter(silent bool) pack.ProgressReporter {
	if silent {
		return &pack.DiscardReporter{}
	}
	return pack.ProgressReporterFn(PrintTransferProgress)
}

// PrintTransferProgress is a helper function that prints incremental
// progress when we are pushing or pulling packages from
// remote repositories
func PrintTransferProgress(current, target int64) {
	fmt.Fprintf(os.Stdout, "\r%v %v/%v", utils.ProgressBar(current, target),
		humanize.Bytes(uint64(current)), humanize.Bytes(uint64(target)))
	if current == target {
		fmt.Fprintf(os.Stdout, "\ntransfer completed\n")
	}
}
