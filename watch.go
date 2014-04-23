package main

import (
	"github.com/philips/focaccia/updateengine"
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

func watch(ch chan bool) {
	for {
		needed := <-ch
		if needed {
			println("Reboot needed")
		}
	}
}

func runWatch(args []string) int {
	var ch chan bool

	ue, err := updateengine.New()
	if err != nil {
		panic(err)
	}

	println(ue.GetStatus())
	go ue.RebootNeededSignal(ch)
	go watch(ch)

	return 0
}
