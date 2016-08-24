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

package main

import (
	"fmt"
	"os"

	"github.com/coreos/go-systemd/login1"
	"github.com/coreos/locksmith/lock"
	"github.com/coreos/locksmith/pkg/machineid"
)

var (
	cmdReboot = &Command{
		Name:        "reboot",
		Summary:     "Reboot honoring reboot locks.",
		Description: `Reboot will attempt to reboot immediately after taking a reboot lock. The user is responsible for unlocking after a successful reboot.`,
		Run:         runReboot,
	}
)

func runReboot(args []string) int {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "Must be root to initiate reboot.")
		return 1
	}

	elc, err := getClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error initializing etcd client:", err)
		return 1
	}

	lgn, err := login1.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error initializing login connection:", err)
		return 1
	}

	mID := machineid.MachineID("/")
	if mID == "" {
		fmt.Fprintln(os.Stderr, "Cannot read machine-id")
		return 1
	}

	l := lock.New(mID, elc)

	err = l.Lock()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error locking:", err)
		return 1
	}

	lgn.Reboot(false)

	// TODO(philips): Unlock if the reboot fails.

	return 0
}
