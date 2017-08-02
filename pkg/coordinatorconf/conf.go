// Copyright 2017 CoreOS, Inc.
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

// Package coordinatorconf is meant to be used by update coordinators, such as
// locksmithd, to update the update coordinator metadata file.
// This file exists at a well-known location and serves as a means for
// coordinators to communicate that they are responsible for updates (via the
// 'NAME' field), and optionally what their status is.
//
// This file lives at the well known locatoin "/run/update-engine/coordinator.conf"
// It is a key=value formatted file which should be safely bash-sourceable.
// In practice, it's expected that neither keys nor values have spaces in them nor take on arbitrary values.
//
// The "NAME" key MUST be set. (e.g. `NAME=locksmithd`). The "STATUS" key should generally bet set.
// The STRATEGY key may optionally be set depending on the coordinator.
package coordinatorconf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coreos/locksmith/pkg/filelock"
)

const UpdateCoordinatorConfPath = "/run/update-engine/coordinator.conf"

type updateCoordinatorState string

const (
	CoordinatorStateStarting updateCoordinatorState = "starting"
	// CoordinatorStateDisabled indicates the coordinator is running, but is
	// intentionally not rebooting, likely due to user configuration
	CoordinatorStateDisabled = "disabled"
	// CoordinatorStateRunning indicates the normal operation of the coordinator
	CoordinatorStateRunning = "running"
	// CoordinatorStateRebootPlanned indicates a reboot will occur once some
	// condition is met (such as a lock being available)
	CoordinatorStateRebootPlanned = "reboot-planned"
	// CoordinatorStateRebooting indicates a reboot has been requested
	CoordinatorStateRebooting = "rebooting"
)

type CoordinatorConfigUpdater interface {
	UpdateState(updateCoordinatorState) error
}

// Implements CoordinatorConfigUpdater
type coordinator struct {
	lock *filelock.UpdateableFileLock

	configLock sync.Mutex
	config     keyValueConf
}

type keyValueConf map[string]string

func (k keyValueConf) String() string {
	parts := make([]string, 0, len(k))
	for key, val := range k {
		parts = append(parts, fmt.Sprintf("%s=%s", key, val))
	}
	return strings.Join(parts, "\n")
}

// New creates a new CoordinatorConfigUpdater. The "name" must be specified.
// Strategy can safely be left blank if desired.
// New will implicitly set the update coordinator state to
// "CoordinatorStateStarting"
func New(name string, strategy string) (CoordinatorConfigUpdater, error) {
	// filelock expects that the file already exists.
	// Because this file will never be deleted, just blindly creating it on every
	// run is safe from racing with any deletions. Just create it, then get
	// locking.
	if err := os.MkdirAll(filepath.Dir(UpdateCoordinatorConfPath), 0755); err != nil {
		return nil, err
	}
	if file, err := os.OpenFile(UpdateCoordinatorConfPath, os.O_CREATE, 0644); err != nil {
		return nil, err
	} else {
		file.Close()
	}

	lock, err := filelock.NewExclusiveLock(UpdateCoordinatorConfPath)
	if err != nil {
		return nil, err
	}
	coordinator := &coordinator{
		lock: lock,
		config: map[string]string{
			"NAME":     name,
			"STRATEGY": strategy,
		},
	}
	err = coordinator.UpdateState(CoordinatorStateStarting)
	return coordinator, err
}

// UpdateState updates the state the update coordinator claims to be in
func (c *coordinator) UpdateState(s updateCoordinatorState) error {
	c.configLock.Lock()
	c.config["STATE"] = string(s)
	c.configLock.Unlock()

	return c.writeConfig()
}

func (c *coordinator) writeConfig() error {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	return c.lock.Update(strings.NewReader(c.config.String()))
}
