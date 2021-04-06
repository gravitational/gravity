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
	"os"

	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/dustin/go-humanize"
)

// ProgressReporter returns new progress reporter either silent
// or verbose based on the settings
func ProgressReporter(silent bool) pack.ProgressReporter {
	if silent {
		return pack.DiscardReporter
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
