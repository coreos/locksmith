package main

import (
	"fmt"
	"os"

	"github.com/coreos/locksmith/lock"
	"github.com/coreos/locksmith/pkg/machineid"
)

var (
	cmdLock = &Command{
		Name:    "lock",
		Summary: "Lock this machine or a given machine-id for reboot.",
		Usage:   "<machine-id>",
		Description: `Lock is for manual locking of the reboot lock for this machine or a given
machine-id. Under normal operation this should not be necessary.`,
		Run: runLock,
	}
)

func runLock(args []string) (exit int) {
	elc, _ := lock.NewEtcdLockClient(nil)

	var mID string

	if len(args) == 0 {
		mID = machineid.MachineID("/")
		if mID == "" {
			fmt.Fprintln(os.Stderr, "Cannot read machine-id")
			return 1
		}
	} else {
		mID = args[0]
	}

	l := lock.New(mID, elc)

	err := l.Lock()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error locking:", err)
		return 1
	}

	return 0
}
