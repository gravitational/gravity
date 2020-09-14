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
	"io/ioutil"
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

var DiscardPrinter = nopPrinter{}

func (nopPrinter) Write(p []byte) (int, error)                               { return 0, nil }
func (nopPrinter) Printf(format string, args ...interface{}) (int, error)    { return 0, nil }
func (nopPrinter) Print(args ...interface{}) (int, error)                    { return 0, nil }
func (nopPrinter) Println(args ...interface{}) (int, error)                  { return 0, nil }
func (nopPrinter) PrintStep(format string, args ...interface{}) (int, error) { return 0, nil }

type nopPrinter struct {
}

// PrintProgress prints generic progress with stages
func PrintProgress(current, target int, message string) {
	fmt.Fprintf(os.Stdout, "\r%v %v%% (%v)      ",
		ProgressBar(int64(current), int64(target)), current, message)
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
		return DiscardProgress
	}
	return NewConsoleProgress(ctx, title, steps)
}

// NewConsoleProgress returns new instance of progress reporter
// steps is the total amount of steps this progress reporter
// will report.
func NewConsoleProgress(ctx context.Context, title string, steps int) *progressPrinter {
	return NewProgressWithConfig(ctx, title, ProgressConfig{Steps: steps})
}

// NewProgressWithConfig returns new progress reporter for the given set of options
func NewProgressWithConfig(ctx context.Context, title string, config ProgressConfig) *progressPrinter {
	config.setDefaults()
	p := &progressPrinter{
		title:   title,
		start:   time.Now(),
		context: ctx,
		timeout: config.Timeout,
		steps:   config.Steps,
		w:       config.Output,
	}
	return p
}

func (r *ProgressConfig) setDefaults() {
	const progressMaxTimeout = 10 * time.Second
	if r.Steps == 0 {
		r.Steps = -1
	}
	if r.Timeout == 0 {
		r.Timeout = progressMaxTimeout
	}
	if r.Output == nil {
		r.Output = &consoleOutput{}
	}
}

// ProgressConfig defines configuration for the progress printer
type ProgressConfig struct {
	// Steps specifies the total number of steps.
	// No steps will be displayed if unspecified
	Steps int
	// Timeout specifies the alotted time.
	// Defaults to progressMaxTimeout if unspecified
	Timeout time.Duration
	// Output specifies the output sink.
	// Defaults to os.Stdout if unspecified
	Output io.Writer
}

// progressPrinter implements Progress that outputs
// to the specified writer
type progressPrinter struct {
	w io.Writer
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
func (p *progressPrinter) PrintCurrentStep(message string, args ...interface{}) {
	entry := p.updateCurrentEntry(message, args...)
	printStep(p.w, entry.current, p.steps, entry.message)
}

// PrintSubStep outputs the message as a sub-step of the current step
func (p *progressPrinter) PrintSubStep(message string, args ...interface{}) {
	entry := p.updateCurrentEntry(message, args...)
	fmt.Fprintf(p.w, "\t%v\n", entry.message)
}

func (p *progressPrinter) updateCurrentEntry(message string, args ...interface{}) entry {
	message = fmt.Sprintf(message, args...)
	var entry entry
	p.Lock()
	p.currentEntry.message = message
	entry = *p.currentEntry
	p.Unlock()
	return entry
}

// Print outputs the specified message in regular color
func (p *progressPrinter) Print(message string, args ...interface{}) {
	printStep(p.w, 0, 0, fmt.Sprintf(message, args...))
}

// PrintInfo outputs the specified info message in color
func (p *progressPrinter) PrintInfo(message string, args ...interface{}) {
	printStep(p.w, 0, 0, color.BlueString(fmt.Sprintf(message, args...)))
}

// PrintWarn outputs the specified warning message in color and logs the error
func (p *progressPrinter) PrintWarn(err error, message string, args ...interface{}) {
	printStep(p.w, 0, 0, color.YellowString(fmt.Sprintf(message, args...)))
	if err != nil {
		logrus.Warnf("%v: %v", fmt.Sprintf(message, args...), err)
	}
}

func (p *progressPrinter) printPeriodic(current int, message string, ctx context.Context) {
	start := time.Now()
	printStep(p.w, current, p.steps, message)

	go func() {
		ticker := time.NewTicker(p.timeout)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				diff := humanize.RelTime(start, time.Now(), "elapsed", "elapsed")
				fmt.Fprintf(p.w, "\tStill %v (%v)\n", lowerFirst(message), diff)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Write outputs p to console
func (r *consoleOutput) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

// consoleOutput outputs to console
type consoleOutput struct{}

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
func (p *progressPrinter) UpdateCurrentStep(message string, args ...interface{}) {
	p.Lock()
	defer p.Unlock()

	if p.currentEntry == nil {
		return
	}
	p.currentEntry.message = fmt.Sprintf(message, args...)
}

// NextStep prints information about next step. It also prints
// updates on the current step if it takes longer than default timeout
func (p *progressPrinter) NextStep(message string, args ...interface{}) {
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
func (p *progressPrinter) Stop() {
	p.Lock()
	defer p.Unlock()

	if p.currentEntry == nil {
		return
	}
	p.currentEntry.cancel()
	if p.steps <= 0 {
		diff := humanize.RelTime(p.start, time.Now(), "", "")
		printStep(p.w, p.currentEntry.current, p.steps, fmt.Sprintf("%v finished in %v", p.title, diff))
	} else if p.currentEntry.current == p.steps {
		diff := humanize.RelTime(p.start, time.Now(), "", "")
		printStep(p.w, p.currentEntry.current, p.steps, fmt.Sprintf("%v completed in %v", p.title, diff))
	} else {
		diff := humanize.RelTime(p.start, time.Now(), "", "")
		printStep(p.w, p.currentEntry.current, p.steps, fmt.Sprintf("%v aborted after %v", p.title, diff))
	}
	p.currentEntry = nil
}

// printStep prints step instead of the progress bar
func printStep(out io.Writer, current, target int, message string) {
	if target > 0 {
		fmt.Fprintf(out, "* [%v/%v] %v\n", current, target, message)
	} else {
		fmt.Fprintf(out, "%v\n", message)
	}
}

// DiscardProgress is a progress reporter that discards all progress output
var DiscardProgress Progress = &nopProgress{}

// nopProgress is a progress printer that reports nothing
type nopProgress struct{}

// UpdateCurrentStep updates message printed for current step that is in progress
func (*nopProgress) UpdateCurrentStep(message string, args ...interface{}) {}

// NextStep prints information about next step. It also prints
// updates on the current step if it takes longer than default timeout
func (*nopProgress) NextStep(message string, args ...interface{}) {}

// Stop stops printing all updates
func (*nopProgress) Stop() {}

// PrintCurrentStep updates and prints current step
func (*nopProgress) PrintCurrentStep(message string, args ...interface{}) {}

// PrintSubStep outputs the message as a sub-step of the current step
func (*nopProgress) PrintSubStep(message string, args ...interface{}) {}

// Print outputs the specified message in regular color
func (*nopProgress) Print(message string, args ...interface{}) {}

// PrintInfo outputs the specified info message in color
func (*nopProgress) PrintInfo(message string, args ...interface{}) {}

// PrintWarn outputs the specified warning message in color and logs the error
func (*nopProgress) PrintWarn(err error, message string, args ...interface{}) {}

// DiscardingLog is a logger that discards output
var DiscardingLog = newDiscardingLogger()

func newDiscardingLogger() (logger *logrus.Logger) {
	logger = logrus.New()
	logger.Out = ioutil.Discard
	return logger
}
