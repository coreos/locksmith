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

package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrExist is the error returned if a holder with the specified id
	// is already holding the semaphore
	ErrExist = errors.New("holder exists")
	// ErrNotExist is the error returned if there is no holder with the
	// specified id holding the semaphore
	ErrNotExist = errors.New("holder does not exist")
)

// Semaphore is a struct representation of the information held by the semaphore
type Semaphore struct {
	Index     uint64   `json:"-"`
	Semaphore int      `json:"semaphore"`
	Max       int      `json:"max"`
	Holders   []string `json:"holders"`
}

// SetMax sets the maximum number of holders of the semaphore
func (s *Semaphore) SetMax(max int) error {
	diff := s.Max - max

	s.Semaphore = s.Semaphore - diff
	s.Max = s.Max - diff

	return nil
}

// String returns a json representation of the semaphore
// if there is an error when marshalling the json, it is ignored and the empty
// string is returned.
func (s *Semaphore) String() string {
	b, _ := json.Marshal(s)
	return string(b)
}

// addHolder adds a holder with id h to the list of holders in the semaphore
// it returns ErrExist if the given id is in the list
func (s *Semaphore) addHolder(h string) error {
	loc := sort.SearchStrings(s.Holders, h)
	switch {
	case loc == len(s.Holders):
		s.Holders = append(s.Holders, h)
	case s.Holders[loc] == h:
		return ErrExist
	default:
		s.Holders = append(s.Holders[:loc], append([]string{h}, s.Holders[loc:]...)...)
	}

	return nil
}

// removeHolder removes a holder with id h from the list of holders in the
// semaphore. It returns ErrNotExist if the given id is not in the list
func (s *Semaphore) removeHolder(h string) error {
	loc := sort.SearchStrings(s.Holders, h)
	if loc < len(s.Holders) && s.Holders[loc] == h {
		s.Holders = append(s.Holders[:loc], s.Holders[loc+1:]...)
	} else {
		return ErrNotExist
	}

	return nil
}

// Lock adds a holder with id h to the semaphore
// It adds the id h to the list of holders, returning ErrExist the id already
// exists, then it subtracts one from the semaphore. If the semaphore is already
// held by the maximum number of people it returns an error.
func (s *Semaphore) Lock(h string) error {
	if s.Semaphore <= 0 {
		return fmt.Errorf("semaphore is at %v", s.Semaphore)
	}

	if err := s.addHolder(h); err != nil {
		return err
	}

	s.Semaphore = s.Semaphore - 1

	return nil
}

// Unlock removes a holder with id h from the semaphore
// It removes the id h from the list of holders, returning ErrNotExist if the id
// does not exist in the list, then adds one to the semaphore.
func (s *Semaphore) Unlock(h string) error {
	if err := s.removeHolder(h); err != nil {
		return err
	}

	s.Semaphore = s.Semaphore + 1

	return nil
}

func newSemaphore() (sem *Semaphore) {
	return &Semaphore{0, 1, 1, nil}
}

type holder struct {
	ID        string `json:"-"`
	StartTime int64  `json:"startTime"`
}
