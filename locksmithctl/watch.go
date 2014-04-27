package main

import (
	"fmt"

	"github.com/coreos/locksmith/updateengine"
)

var (
	cmdWatch = &Command{
		Name:    "watch",
		Summary: "Watch for reboot needed signal and if reboot able.",
		Usage:   "UNIT",
		Description: `Watch waits for the reboot needed signal coming out of update engine and
attempts to acquire the reboot lock. If the reboot lock is acquired then the
machine will reboot.`,
		Run: runWatch,
	}
)

func runWatch(args []string) int {
	ch := make(chan updateengine.Status, 1)

	ue, err := updateengine.New()
	if err != nil {
		panic(err)
	}

	result, err := ue.GetStatus()
	if err == nil {
		fmt.Println(result.String())
	}

	go ue.RebootNeededSignal(ch)
	status := <-ch
	// TODO(philips): use the logind dbus interfaces to do this
	println("Reboot needed!", status.String())


	return 0
}
