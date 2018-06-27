// Copyright 2015 CoreOS, Inc.
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

package updateengine

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus"
)

// Status is a struct containing the information passed by updateengine on every
// status update.
type Status struct {
	LastCheckedTime  int64
	Progress         float64
	CurrentOperation string
	NewVersion       string
	NewSize          int64
}

var (
	errStatusNil    = errors.New("dbus signal is nil")
	errStatusLength = errors.New("signal body has insufficient length")
	errStatusType   = errors.New("signal body has incorrect type")
)

// TryNewStatus attempts to create a new Status from a list of fields in
// the dbus.Signal. An error is returned if the signal is nil, has less fields
// than required, or has a field of incorrect type (types must be int64, float64,
// string, string, int64 respectively). If successful, the new status is returned.
func TryNewStatus(signal *dbus.Signal) (Status, error) {
	if signal == nil {
		return Status{}, errStatusNil
	}

	if len(signal.Body) < 5 {
		return Status{}, errStatusLength
	}

	var ok bool
	_, ok = signal.Body[0].(int64)
	if !ok {
		return Status{}, errStatusType
	}

	_, ok = signal.Body[1].(float64)
	if !ok {
		return Status{}, errStatusType
	}

	_, ok = signal.Body[2].(string)
	if !ok {
		return Status{}, errStatusType
	}

	_, ok = signal.Body[3].(string)
	if !ok {
		return Status{}, errStatusType
	}

	_, ok = signal.Body[4].(int64)
	if !ok {
		return Status{}, errStatusType
	}

	return NewStatus(signal.Body), nil
}

// NewStatus creates a new status from a list of fields.
// this function will panic at runtime if there are less than five elements in
// the provided list, or if they are not typed int64, float64, string, string,
// int64 respectively.
func NewStatus(body []interface{}) (s Status) {
	s.LastCheckedTime = body[0].(int64)
	s.Progress = body[1].(float64)
	s.CurrentOperation = body[2].(string)
	s.NewVersion = body[3].(string)
	s.NewSize = body[4].(int64)

	return
}

// String returns a string representation of the Status struct.
// It is of the form
// "LastCheckedTime=%v Progress=%v CurrentOperation=%q NewVersion=%v NewSize=%v"
func (s *Status) String() string {
	return fmt.Sprintf("LastCheckedTime=%v Progress=%v CurrentOperation=%q NewVersion=%v NewSize=%v",
		s.LastCheckedTime,
		s.Progress,
		s.CurrentOperation,
		s.NewVersion,
		s.NewSize,
	)
}
