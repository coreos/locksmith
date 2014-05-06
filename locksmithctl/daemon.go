package main

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/coreos/locksmith/third_party/github.com/coreos/go-systemd/dbus"
	"github.com/coreos/locksmith/third_party/github.com/coreos/go-systemd/login1"

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
	maxTimeout     = time.Minute * 5
)

const (
	StrategyReboot     = "reboot"
	StrategyEtcdLock   = "etcd-lock"
	StrategyBestEffort = "best-effort"
)

func expBackoff(try int) time.Duration {
	sleep := time.Duration(math.Pow(2, float64(try))) * initialTimeout
	if sleep > maxTimeout {
		sleep = maxTimeout
	}

	return sleep
}

func rebootAndSleep(lgn *login1.Conn) {
	lgn.Reboot(false)
	fmt.Println("Reboot sent. Going to sleep.")

	// Wait a really long time for the reboot to occur.
	time.Sleep(time.Hour * 24 * 7)
}

// lockAndReboot attempts to acquire the lock and reboot the machine in an
// infinite loop. Returns if the reboot failed.
func (r rebooter) lockAndReboot(lck *lock.Lock) {
	tries := 0
	for {
		err := lck.Lock()
		if err != nil && err != lock.ErrExist {
			sleep := expBackoff(tries)
			fmt.Printf("Retrying in %v. Error locking: %v\n", sleep, err)
			time.Sleep(sleep)
			tries = tries + 1

			continue
		}

		rebootAndSleep(r.lgn)

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

type rebooter struct {
	strategy string
	lgn *login1.Conn
}

func (r rebooter) useLock() (useLock bool, err error) {
	switch r.strategy {
	case "":
		fallthrough
	case StrategyBestEffort:
		running, err := etcdActive()
		if err != nil {
			return false, err
		}
		if running {
			fmt.Println("etcd.service is active")
			useLock = true
		} else {
			fmt.Println("etcd.service is inactive")
			useLock = false
		}
	case StrategyEtcdLock:
		useLock = true
	case StrategyReboot:
		useLock = false
	default:
		return false, fmt.Errorf("Unknown strategy: %s", r.strategy)
	}

	return useLock, nil
}

func (r rebooter) reboot() int {
	useLock, err := r.useLock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if useLock {
		lck, err := setupLock()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		r.lockAndReboot(lck)
	}

	rebootAndSleep(r.lgn)
	fmt.Println("Error: reboot attempt never finished")
	return 1
}

func runDaemon(args []string) int {
	strategy := os.ExpandEnv("${LOCKSMITH_STRATEGY}")

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


	ch := make(chan updateengine.Status, 1)
	go ue.RebootNeededSignal(ch)

	r := rebooter{strategy, lgn}

	result, err := ue.GetStatus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot get update engine status:", err)
		return 1
	}

	if result.CurrentOperation == updateengine.UpdateStatusUpdatedNeedReboot {
		return r.reboot()
	}

	fmt.Printf("locksmithd starting currentOperation=%q strategy=%q\n",
		result.CurrentOperation,
		os.ExpandEnv("${LOCKSMITH_STRATEGY}"),
	)

	// Wait for a reboot needed signal
	<-ch
	return r.reboot()
}
