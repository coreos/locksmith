package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/coreos/locksmith/third_party/github.com/godbus/dbus"
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
