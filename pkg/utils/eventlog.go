// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"regexp"
	"runtime"

	v1 "k8s.io/api/core/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// QuiteLogLel - "important" information
const QuiteLogLel = 4

// NoiseLogLel - information inside "important functions"
const NoiseLogLel = 5

// VeryNoisy = show call stack, routine  and everything
const VeryNoisy = 10

var regexStripFnPreamble = regexp.MustCompile(`^.*\.(.*)$`)

// GetFnName - get name of function
func GetFnName() string {
	fnName := "<unknown>"
	// Skip this function, and fetch the PC and file for its parent
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		fnName = regexStripFnPreamble.ReplaceAllString(runtime.FuncForPC(pc).Name(), "$1")
	}
	return fnName
}

// EnterFnString - called when enter a function
func EnterFnString() string {
	return ""
}

// ExitFuString - called when exiting a function
func ExitFuString(s string) {}

// RecordEvent - record kuberentes event
func RecordEvent(eventrecorder record.EventRecorder, obj apiruntime.Object, reason, msg string, err error) {
	eventType := v1.EventTypeNormal
	evnetMsg := msg

	if err != nil {
		eventType = v1.EventTypeWarning
	}

	eventrecorder.Event(obj, eventType, reason, evnetMsg)
}
