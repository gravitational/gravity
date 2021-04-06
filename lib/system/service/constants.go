/*
Copyright 2020 Gravitational, Inc.

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

package service

// Systemd service unit configuration constants
// More documentation on configuration can be found here:
// https://www.freedesktop.org/software/systemd/man/systemd.service.html
const (
	// OneshotService is a service that executes one time
	OneshotService = "oneshot"

	// SimpleService is a simple service that is recommended for long running processes
	SimpleService = "simple"

	// RestartOnFailure defines the restart on-failure rule for a service
	RestartOnFailure = "on-failure"
)
