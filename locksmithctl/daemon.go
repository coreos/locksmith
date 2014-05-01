package main

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/coreos/go-systemd/dbus"
	"github.com/coreos/go-systemd/login1"

	"github.com/coreos/locksmith/lock"
	"github.com/coreos/locksmith/pkg/machineid"
	"github.com/coreos/locksmith/updateengine"
)

var (
	cmdDaemon = &Command{
		Name:        "daemon",
		Summary:     "Daemon for reboot needed signal and if reboot able.",
		Description: `Daemon waits for the reboot needed signal coming out of update engine and attempts to acquire the reboot lock. If the reboot lock is acquired then the machine will reboot.`,
		Run:         runDaemon,
	}
)

const (
	initialTimeout = time.Second * 5
	maxTimeout     = time.Minute * 30
)

const (
	StrategyReboot     = "reboot"      // Reboot immediatly.
	StrategyEtcdLock   = "etcd-lock"   // Always take a lock from etcd.
	StrategyBestEffort = "best-effort" // If etcd is running then lock.
)

func expBackoff(try int) time.Duration {
	sleep := time.Duration(math.Pow(2, float64(try))) * initialTimeout
	if sleep > maxTimeout {
		sleep = maxTimeout
	}

	return sleep
}

// lockAndReboot attempts to acquire the lock and reboot the machine in an
// infinite loop. Returns if the lock was acquired and the reboot worked.
func lockAndReboot(lck *lock.Lock, lgn *login1.Conn) {
	tries := 0
	for {
		err := lck.Lock()
		if err != nil && err != lock.ErrExist {
			sleep := expBackoff(tries)
			fmt.Println("Retrying in %v. Error locking: %v", sleep, err)
			time.Sleep(sleep)
			tries = tries + 1

			continue
		}

		lgn.Reboot(false)
		fmt.Println("Reboot signal sent.")

		// Wait a really long time for the reboot to occur.
		time.Sleep(time.Hour * 24 * 7)

		return
	}
}

func unlockIfHeld(lck *lock.Lock) {
	tries := 0
	for {
		err := lck.Unlock()
		if err == nil {
			fmt.Println("Unlocked existing lock for this machine")
			return
		} else if err == lock.ErrNotExist {
			return
		}

		sleep := expBackoff(tries)
		fmt.Println("Retrying in %v. Error unlocking: %v", sleep, err)
		time.Sleep(sleep)
		tries = tries + 1
	}
}

func setupLock() (lck *lock.Lock, err error) {
	elc, err := lock.NewEtcdLockClient(nil)
	if err != nil {
		return nil, fmt.Errorf("Error initializing etcd client: %v", err)
	}

	mID := machineid.MachineID("/")
	if mID == "" {
		return nil, fmt.Errorf("Cannot read machine-id")
	}

	lck = lock.New(mID, elc)

	unlockIfHeld(lck)

	return lck, nil
}

// etcdActive returns true if etcd is not in an inactive state according to systemd.
func etcdActive() (running bool, err error) {
	sys, err := dbus.New()
	if err != nil {
		return false, err
	}

	prop, err := sys.GetUnitProperty("etcd.service", "ActiveState")
	if err != nil {
		return false, fmt.Errorf("Error getting etcd.service ActiveState: %v", err)
	}

	if prop.Value.Value().(string) == "inactive" {
		return false, nil
	}

	return true, nil
}

func runDaemon(args []string) int {
	var lck *lock.Lock

	useLock := false
	switch s := os.ExpandEnv("${LOCKSMITH_STRATEGY}"); {
	case s == StrategyEtcdLock:
		useLock = true
	case s == StrategyBestEffort:
		running, err := etcdActive()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if running {
			fmt.Println("etcd.service is active")
			useLock = true
		} else {
			fmt.Println("etcd.service is inactive")
			useLock = false
		}
	case s == StrategyReboot:
		useLock = false
	case s == "":
		useLock = false
	default:
		fmt.Fprintln(os.Stderr, "Unknown strategy:", s)
		return 1
	}

	ue, err := updateengine.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error initializing update1 client:", err)
		return 1
	}

	lgn, err := login1.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error initializing login1 client:", err)
		return 1
	}

	if useLock {
		lck, err = setupLock()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	result, err := ue.GetStatus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot get update engine status:", err)
		return 1
	}

	if result.CurrentOperation == updateengine.UpdateStatusUpdatedNeedReboot {
		lockAndReboot(lck, lgn)
		return 0
	}

	fmt.Printf("locksmithd starting currentOperation=%q strategy=%q useLock=%t\n",
		result.CurrentOperation,
		os.ExpandEnv("${LOCKSMITH_STRATEGY}"),
		useLock,
	)

	ch := make(chan updateengine.Status, 1)

	go ue.RebootNeededSignal(ch)
	<-ch

	if useLock {
		lockAndReboot(lck, lgn)
	}

	lgn.Reboot(false)

	return 0
}
