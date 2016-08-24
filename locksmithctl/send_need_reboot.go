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
	"strconv"
	"time"

	"github.com/godbus/dbus"
)

var (
	cmdSendNeedReboot = &Command{
		Name:    "send-need-reboot",
		Summary: "send a 'need reboot' signal over dbus.",
		Description: `Send a fake 'need reboot' signal to test locksmithd
without a full update cycle via update engine.`,
		Run: runSendNeedReboot,
	}
)

func runSendNeedReboot(args []string) int {
	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error connecting to system dbus:", err)
		return 1
	}

	methods := []dbus.Auth{dbus.AuthExternal(strconv.Itoa(os.Getuid()))}
	err = conn.Auth(methods)
	if err != nil {
		conn.Close()
		fmt.Fprintln(os.Stderr, "error authing to system dbus:", err)
		return 1
	}

	err = conn.Hello()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error sending hello:", err)
		conn.Close()
		return 1
	}

	err = conn.Emit("/com/coreos/update1",
		"com.coreos.update1.Manager.StatusUpdate",
		int64(0), float64(0),
		string("UPDATE_STATUS_UPDATED_NEED_REBOOT"),
		string(""), int64(0))

	if err != nil {
		fmt.Fprintln(os.Stderr, "error emitting signal:", err)
		conn.Close()
		return 1
	}

	// TODO(philips): figure out a way to make conn.Close() block until
	// everything is flushed.
	time.Sleep(time.Second)
	conn.Close()

	return 0
}
