package main

import (
	"fmt"
	"os"

	"github.com/coreos/locksmith/lock"
	"github.com/coreos/locksmith/pkg/machineid"
)

var (
	cmdUnlock = &Command{
		Name:    "unlock",
		Summary: "Unlock this machine or a given machine-id for reboot.",
		Usage:   "<machine-id>",
		Description: `Unlock is for manual unlocking of the reboot unlock for this machine or a
given machine-id. Under normal operation this should not be necessary.`,
		Run: runUnlock,
	}
)

func runUnlock(args []string) (exit int) {
	elc, err := lock.NewEtcdLockClient(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error initializing etcd client:", err)
		return 1
	}

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

	err = l.Unlock()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error unlocking:", err)
		return 1
	}

	return 0
}
