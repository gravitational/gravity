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

package phases

const (
	// InitPhase identifies the initialization phase that resets services
	// back to their original values on rollback
	InitPhase = "init"
	// FiniPhase identifies the finalization phase that commits
	// changes after a successful operation
	FiniPhase = "fini"
)
