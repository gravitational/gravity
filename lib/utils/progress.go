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

package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/constants"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

// Printer describes a capability to output to standard output
type Printer interface {
	io.Writer
	Printf(format string, args ...interface{}) (int, error)
	Print(args ...interface{}) (int, error)
	Println(args ...interface{}) (int, error)
	PrintStep(format string, args ...interface{}) (int, error)
}

// PrintProgress prints generic progress with stages
func PrintProgress(current, target int, message string) {
	fmt.Fprintf(os.Stdout, "\r%v %v%% (%v)      ",
		ProgressBar(int64(current), int64(target)), current, message)
}

// PrintStep prints step instead of the progress bar
func PrintStep(current, target int, message string) {
	if target > 0 {
		fmt.Fprintf(os.Stdout, "* [%v/%v] %v\n", current, target, message)
	} else {
		fmt.Fprintf(os.Stdout, "%v\t%v\n", time.Now().Format(
			constants.HumanDateFormatSeconds), message)
	}
}

func ProgressBar(current, target int64) string {
	if target == 0 {
		target = 1
	}
	ratio := float64(current) / float64(target)
	blocks := int(ratio * constants.Completed)
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "[")
	for i := 0; i < constants.Completed; i++ {
		if blocks-i > 0 {
			fmt.Fprintf(b, "=")
		} else if blocks-i == 0 {
			fmt.Fprintf(b, ">")
		} else {
			fmt.Fprintf(b, " ")
		}
	}
	fmt.Fprintf(b, "]")
	return b.String()
}

type entry struct {
	message string
	current int
	cancel  context.CancelFunc
	context context.Context
}

// Progress is a progress reporter
type Progress interface {
	// UpdateCurrentStep updates message printed for current step that is in progress
	UpdateCurrentStep(message string, args ...interface{})
	// NextStep prints information about next step. It also prints
	// updates on the current step if it takes longer than default timeout
	NextStep(message string, args ...interface{})
	// Stop stops printing all updates
	Stop()
	// PrintCurrentStep updates and prints current step
	PrintCurrentStep(message string, args ...interface{})
	// PrintSubStep outputs the message as a sub-step of the current step
	PrintSubStep(message string, args ...interface{})
	// Print outputs the specified message in regular color
	Print(message string, args ...interface{})
	// PrintInfo outputs the specified info message in color
	PrintInfo(message string, args ...interface{})
	// PrintWarn outputs the specified warning message in color and logs the error
	PrintWarn(err error, message string, args ...interface{})
}

// NewProgress returns new instance of progress reporter
// based on verbosity - returns either console printer or discarding progress
//
// If negative total number of steps is provided, it means amount of steps is unknown
// beforehand and the step numbers will not be printed.
func NewProgress(ctx context.Context, title string, steps int, silent bool) Progress {
	if silent {
		return NewNopProgress()
	}
	return NewConsoleProgress(ctx, title, steps)
}

// NewConsoleProgress returns new instance of progress reporter
// steps is the total amount of steps this progress reporter
// will report.
func NewConsoleProgress(ctx context.Context, title string, steps int) *ConsoleProgress {
	return &ConsoleProgress{
		title:   title,
		start:   time.Now(),
		timeout: 10 * time.Second,
		context: ctx,
		steps:   steps,
	}
}

// ConsoleProgress is a helper progress printer
// that prints all the output to the console
type ConsoleProgress struct {
	sync.Mutex
	title        string
	currentEntry *entry
	timeout      time.Duration
	steps        int
	currentStep  int
	context      context.Context
	start        time.Time
}

// PrintCurrentStep updates message printed for current step that is in progress
func (p *ConsoleProgress) PrintCurrentStep(message string, args ...interface{}) {
	entry := p.updateCurrentEntry(message, args...)
	PrintStep(entry.current, p.steps, entry.message)
}

// PrintSubStep outputs the message as a sub-step of the current step
func (p *ConsoleProgress) PrintSubStep(message string, args ...interface{}) {
	entry := p.updateCurrentEntry(message, args...)
	fmt.Fprintf(os.Stdout, "\t%v\n", entry.message)
}

func (p *ConsoleProgress) updateCurrentEntry(message string, args ...interface{}) *entry {
	message = fmt.Sprintf(message, args...)
	var entry *entry
	p.Lock()
	p.currentEntry.message = message
	entry = p.currentEntry
	p.Unlock()
	return entry
}

// Print outputs the specified message in regular color
func (p *ConsoleProgress) Print(message string, args ...interface{}) {
	PrintStep(0, 0, fmt.Sprintf(message, args...))
}

// PrintInfo outputs the specified info message in color
func (p *ConsoleProgress) PrintInfo(message string, args ...interface{}) {
	PrintStep(0, 0, color.BlueString(fmt.Sprintf(message, args...)))
}

// PrintWarn outputs the specified warning message in color and logs the error
func (p *ConsoleProgress) PrintWarn(err error, message string, args ...interface{}) {
	PrintStep(0, 0, color.YellowString(fmt.Sprintf(message, args...)))
	if err != nil {
		logrus.Warnf("%v: %v", fmt.Sprintf(message, args...), err)
	}
}

func (p *ConsoleProgress) printPeriodic(current int, message string, ctx context.Context) {
	start := time.Now()
	PrintStep(current, p.steps, message)

	go func() {
		ticker := time.NewTicker(p.timeout)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				diff := humanize.RelTime(start, time.Now(), "elapsed", "elapsed")
				fmt.Fprintf(os.Stdout, "\tStill %v (%v)\n", lowerFirst(message), diff)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// lowerFirst returns the copy of the provided string with the first
// character lowercased
func lowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if len(s) == 1 {
		return strings.ToLower(s)
	}
	return strings.Split(strings.ToLower(s), "")[0] + s[1:]
}

// UpdateCurrentStep updates message printed for current step that is in progress
func (p *ConsoleProgress) UpdateCurrentStep(message string, args ...interface{}) {
	p.Lock()
	defer p.Unlock()

	if p.currentEntry == nil {
		return
	}
	p.currentEntry.message = fmt.Sprintf(message, args...)
}

// NextStep prints information about next step. It also prints
// updates on the current step if it takes longer than default timeout
func (p *ConsoleProgress) NextStep(message string, args ...interface{}) {
	p.Lock()
	defer p.Unlock()

	p.currentStep++

	message = fmt.Sprintf(message, args...)

	ctx, cancel := context.WithCancel(p.context)
	entry := &entry{
		current: p.currentStep,
		message: message,
		context: ctx,
		cancel:  cancel,
	}

	if p.currentEntry != nil {
		p.currentEntry.cancel()
	}
	p.currentEntry = entry
	p.printPeriodic(entry.current, entry.message, entry.context)
}

// Stop stops printing all updates
func (p *ConsoleProgress) Stop() {
	p.Lock()
	defer p.Unlock()

	if p.currentEntry != nil {
		p.currentEntry.cancel()
		if p.steps <= 0 {
			diff := humanize.RelTime(p.start, time.Now(), "", "")
			PrintStep(p.currentEntry.current, p.steps, fmt.Sprintf("%v finished in %v", p.title, diff))
		} else if p.currentEntry.current == p.steps {
			diff := humanize.RelTime(p.start, time.Now(), "", "")
			PrintStep(p.currentEntry.current, p.steps, fmt.Sprintf("%v completed in %v", p.title, diff))
		} else {
			diff := humanize.RelTime(p.start, time.Now(), "", "")
			PrintStep(p.currentEntry.current, p.steps, fmt.Sprintf("%v aborted after %v", p.title, diff))
		}
	}
	p.currentEntry = nil
}

// DiscardProgress is a progress reporter that discards all progress output
var DiscardProgress Progress = &NopProgress{}

// NewNopProgress returns an instance of discarding output progress reporter
func NewNopProgress() *NopProgress {
	return &NopProgress{}
}

// NopProgress is a progress printer that reports nothing
type NopProgress struct{}

// UpdateCurrentStep updates message printed for current step that is in progress
func (*NopProgress) UpdateCurrentStep(message string, args ...interface{}) {}

// NextStep prints information about next step. It also prints
// updates on the current step if it takes longer than default timeout
func (*NopProgress) NextStep(message string, args ...interface{}) {}

// Stop stops printing all updates
func (*NopProgress) Stop() {}

// PrintCurrentStep updates and prints current step
func (*NopProgress) PrintCurrentStep(message string, args ...interface{}) {}

// PrintSubStep outputs the message as a sub-step of the current step
func (*NopProgress) PrintSubStep(message string, args ...interface{}) {}

// Print outputs the specified message in regular color
func (*NopProgress) Print(message string, args ...interface{}) {}

// PrintInfo outputs the specified info message in color
func (*NopProgress) PrintInfo(message string, args ...interface{}) {}

// PrintWarn outputs the specified warning message in color and logs the error
func (*NopProgress) PrintWarn(err error, message string, args ...interface{}) {}

// Emitter abstracts a way to emit progess messages within an operation
type Emitter interface {
	// PrintStep formats the specified message string to stdout
	PrintStep(format string, args ...interface{}) (n int, err error)
}

// NopEmitter returns an emitter that does nothing
func NopEmitter() Emitter {
	return nilEmitter{}
}

// PrintStep is a noop
func (nilEmitter) PrintStep(format string, args ...interface{}) (n int, err error) {
	return 0, nil
}

type nilEmitter struct{}
