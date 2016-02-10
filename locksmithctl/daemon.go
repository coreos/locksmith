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

/*
#include <utmp.h>
*/
import "C"

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/locksmith/Godeps/_workspace/src/github.com/coreos/go-systemd/dbus"
	"github.com/coreos/locksmith/Godeps/_workspace/src/github.com/coreos/go-systemd/login1"
	"github.com/coreos/locksmith/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"

	"github.com/coreos/locksmith/lock"
	"github.com/coreos/locksmith/pkg/machineid"
	"github.com/coreos/locksmith/pkg/timeutil"
	"github.com/coreos/locksmith/updateengine"
)

const (
	initialInterval   = time.Second * 5
	maxInterval       = time.Minute * 5
	loginsRebootDelay = time.Minute * 5
)

var (
	etcdServices = []string{
		"etcd.service",
		"etcd2.service",
	}

	// TODO(mischief): daemon is not really a seperate package. it probably should be.
	dlog = capnslog.NewPackageLogger("github.com/coreos/locksmith", "daemon")
)

const (
	StrategyReboot     = "reboot"
	StrategyEtcdLock   = "etcd-lock"
	StrategyBestEffort = "best-effort"
	StrategyOff        = "off"
)

// attempt to broadcast msg to all lines registered in utmp
// returns count of lines successfully opened (and likely broadcasted to)
func broadcast(msg string) uint {
	var cnt uint
	C.setutent()

	for {
		var utmp *C.struct_utmp
		utmp = C.getutent()
		if utmp == nil {
			break
		}

		line := C.GoString(&utmp.ut_line[0])
		tty, _ := os.OpenFile("/dev/"+line, os.O_WRONLY, 0)
		if tty == nil {
			// ignore errors, this is just a best-effort courtesy notice
			continue
		}
		cnt++
		go func() {
			fmt.Fprintf(tty, "\r%79s\r\n", " ")
			fmt.Fprintf(tty, "%-79.79s\007\007\r\n", fmt.Sprintf("Broadcast message from locksmithd at %s:", time.Now()))
			fmt.Fprintf(tty, "%-79.79s\r\n", msg) // msg is assumed to be short and not require wrapping
			fmt.Fprintf(tty, "\r%79s\r\n", " ")
			tty.Close()
		}()
	}

	return cnt
}

func expBackoff(interval time.Duration) time.Duration {
	interval = interval * 2
	if interval > maxInterval {
		interval = maxInterval
	}
	return interval
}

func rebootAndSleep(lgn *login1.Conn) {
	// Broadcast a notice, if broadcast found lines to notify, delay the reboot.
	delaymins := loginsRebootDelay / time.Minute
	lines := broadcast(fmt.Sprintf("System reboot in %d minutes!", delaymins))
	if 0 != lines {
		dlog.Noticef("Logins detected, delaying reboot for %d minutes.", delaymins)
		time.Sleep(loginsRebootDelay)
	}
	lgn.Reboot(false)
	dlog.Info("Reboot sent. Going to sleep.")

	// Wait a really long time for the reboot to occur.
	time.Sleep(time.Hour * 24 * 7)
}

// lockAndReboot attempts to acquire the lock and reboot the machine in an
// infinite loop. Returns if the reboot failed.
func (r rebooter) lockAndReboot(lck *lock.Lock) {
	interval := initialInterval
	for {
		err := lck.Lock()
		if err != nil && err != lock.ErrExist {
			interval = expBackoff(interval)
			dlog.Warningf("Failed to acquire lock: %v. Retrying in %v.", err, interval)
			time.Sleep(interval)

			continue
		}

		rebootAndSleep(r.lgn)

		return
	}
}

func setupLock() (lck *lock.Lock, err error) {
	elc, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("Error initializing etcd client: %v", err)
	}

	mID := machineid.MachineID("/")
	if mID == "" {
		return nil, fmt.Errorf("Cannot read machine-id")
	}

	lck = lock.New(mID, elc)

	return lck, nil
}

// etcdActive returns true if etcd is not in an inactive state according to systemd.
func etcdActive() (active bool, name string, err error) {
	active = false
	name = ""

	sys, err := dbus.New()
	if err != nil {
		return
	}
	defer sys.Close()

	for _, service := range etcdServices {
		prop, err := sys.GetUnitProperty(service, "ActiveState")
		if err != nil {
			continue
		}

		switch prop.Value.Value().(string) {
		case "inactive":
			continue
		default:
			active = true
			name = service
			break
		}
	}

	return
}

type rebooter struct {
	strategy string
	lgn      *login1.Conn
}

func (r rebooter) reboot() int {
	useLock, err := useLock(r.strategy)
	if err != nil {
		dlog.Errorf("Failed to figure out if locksmithd needs to use a lock: %v", err)
		return 1
	}

	if useLock {
		lck, err := setupLock()
		if err != nil {
			dlog.Errorf("Failed to set up lock: %v", err)
			return 1
		}

		err = unlockIfHeld(lck)
		if err != nil {
			dlog.Errorf("Failed to unlock held lock: %v", err)
			return 1
		}

		r.lockAndReboot(lck)
	}

	rebootAndSleep(r.lgn)
	dlog.Fatal("Tried to reboot but did not!")
	return 1
}

// useLock returns whether locksmith should attempt to take a lock before
// rebooting or release a lock afterwards, based on the given strategy.
// If strategy is set to best effort, this will be dependent on whether the
// local instance of etcd is active. Otherwise, the lock will always be
// attempted (in the case of strategy = etcd lock) or never be attempted (in
// the case of strategy = reboot)
func useLock(strategy string) (useLock bool, err error) {
	switch strategy {
	case StrategyBestEffort:
		active, name, err := etcdActive()
		if err != nil {
			return false, err
		}
		if active {
			dlog.Infof("%s is active", name)
			useLock = true
		} else {
			dlog.Infof("%v are inactive", etcdServices)
			useLock = false
		}
	case StrategyEtcdLock:
		useLock = true
	case StrategyReboot:
		useLock = false
	default:
		return false, fmt.Errorf("unknown strategy: %s", strategy)
	}

	return useLock, nil
}

// unlockIfHeld will unlock a lock, if it is held by this machine, or return an error.
func unlockIfHeld(lck *lock.Lock) error {
	err := lck.Unlock()
	if err == lock.ErrNotExist {
		return nil
	} else if err == nil {
		dlog.Info("Unlocked existing lock for this machine")
		return nil
	}

	return err
}

// unlockHeldLock will loop until it can confirm that any held locks are
// released or a stop signal is sent.
func unlockHeldLocks(strategy string, stop chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	interval := initialInterval
	for {
		var reason string
		select {
		case <-stop:
			return
		case <-time.After(interval):
			// Here we assume that the strategy used by locksmith
			// is consistent with that used before the previous
			// reboot (if any), and use that to decide whether to
			// attempt to unlock.
			shouldUnlock, err := useLock(strategy)
			if err != nil {
				reason = fmt.Sprintf("error checking whether lock should be released: %v", err)
				break
			}
			if !shouldUnlock {
				reason = fmt.Sprintf("%v are inactive", etcdServices)
				break
			}

			lck, err := setupLock()
			if err != nil {
				reason = "error setting up lock: " + err.Error()
				break
			}

			err = unlockIfHeld(lck)
			if err == nil {
				return
			}
			reason = err.Error()
		}

		interval = expBackoff(interval)
		dlog.Errorf("Unlocking old locks failed: %v. Retrying in %v.", reason, interval)
	}
}

// runDaemon waits for the reboot needed signal coming out of update engine and
// attempts to acquire the reboot lock. If the reboot lock is acquired then the
// machine will reboot.
func runDaemon() int {
	var period *timeutil.Periodic

	strategy := os.Getenv("REBOOT_STRATEGY")

	if strategy == "" {
		strategy = StrategyBestEffort
	}

	// XXX: complain loudly if besteffort is used
	if strategy == StrategyBestEffort {
		dlog.Errorf("Reboot strategy %q is deprecated and will be removed in the future.", strategy)
		dlog.Errorf("Please explicitly set the reboot strategy to one of %v", []string{StrategyOff, StrategyReboot, StrategyEtcdLock})
		dlog.Error("See https://coreos.com/os/docs/latest/update-strategies.html for details on configuring reboot strategies.")
	}

	if strategy == StrategyOff {
		dlog.Noticef("Reboot strategy is %q - shutting down.", strategy)
		return 0
	}

	// XXX: REBOOT_WINDOW_* are deprecated in favor of variables with LOCKSMITHD_ prefix,
	// but the old ones are read for compatibility.
	startw := os.Getenv("LOCKSMITHD_REBOOT_WINDOW_START")
	if startw == "" {
		startw = os.Getenv("REBOOT_WINDOW_START")
	}

	lengthw := os.Getenv("LOCKSMITHD_REBOOT_WINDOW_LENGTH")
	if lengthw == "" {
		lengthw = os.Getenv("REBOOT_WINDOW_LENGTH")
	}

	if (startw == "") != (lengthw == "") {
		dlog.Fatal("Either both or neither $REBOOT_WINDOW_START and $REBOOT_WINDOW_LENGTH must be set")
	}

	if startw != "" && lengthw != "" {
		p, err := timeutil.ParsePeriodic(startw, lengthw)
		if err != nil {
			dlog.Fatalf("Error parsing reboot window: %s", err)
		}

		period = p
	}

	if period != nil {
		dlog.Infof("Reboot window start is %q and length is %q", startw, lengthw)
		next := period.Next(time.Now())
		dlog.Infof("Next window begins at %s and ends at %s", next.Start, next.End)
	} else {
		dlog.Info("No configured reboot window")
	}

	shutdown := make(chan os.Signal, 1)
	stop := make(chan struct{}, 1)

	go func() {
		<-shutdown
		dlog.Notice("Received interrupt/termination signal - shutting down.")
		os.Exit(0)
	}()
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	ue, err := updateengine.New()
	if err != nil {
		dlog.Fatalf("Error initializing update1 client: %v", err)
	}

	lgn, err := login1.New()
	if err != nil {
		dlog.Fatalf("Error initializing login1 client: %v", err)
	}

	var wg sync.WaitGroup
	if strategy != StrategyReboot {
		wg.Add(1)
		go unlockHeldLocks(strategy, stop, &wg)
	}

	ch := make(chan updateengine.Status, 1)
	go ue.RebootNeededSignal(ch, stop)

	r := rebooter{strategy, lgn}

	result, err := ue.GetStatus()
	if err != nil {
		dlog.Fatalf("Cannot get update engine status: %v", err)
	}

	dlog.Infof("locksmithd starting currentOperation=%q strategy=%q", result.CurrentOperation, strategy)

	if result.CurrentOperation != updateengine.UpdateStatusUpdatedNeedReboot {
		<-ch
	}

	close(stop)
	wg.Wait()

	if period != nil {
		now := time.Now()
		sleeptime := period.DurationToStart(now)
		if sleeptime > 0 {
			dlog.Infof("Waiting for %s to reboot.", sleeptime)
			time.Sleep(sleeptime)
		}
	}

	return r.reboot()
}
