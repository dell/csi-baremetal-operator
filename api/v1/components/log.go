/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package components

// Format is format in which log will appear
type Format string

const (
	// JSONFormat indicates that log are shown in json format
	JSONFormat Format = "json"
	// TextFormat indicates that log are shown in usual format
	TextFormat Format = "text"
)

// Level indicates which types if logging need to be show
type Level string

const (
	// InfoLevel includes Info, Error, Fatal, Warn logs
	InfoLevel Level = "info"
	// debug includes InfoLevel and Debug
	DebugLevel Level = "debug"
	// debug includes InfoLevel, DebugLevel and Trace
	TraceLevel Level = "trace"
)

// Log is a configuration for logger in components
type Log struct {
	Format Format `json:"format"`
	Level  Level  `json:"level"`
}
