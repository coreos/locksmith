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

	"github.com/coreos/go-systemd/login1"
	"github.com/coreos/pkg/capnslog"

	"github.com/coreos/locksmith/lock"
	"github.com/coreos/locksmith/pkg/coordinatorconf"
	"github.com/coreos/locksmith/pkg/machineid"
	"github.com/coreos/locksmith/pkg/timeutil"
	"github.com/coreos/locksmith/updateengine"
)

const (
	initialInterval   = time.Second * 5
	maxInterval       = time.Minute * 5
	loginsRebootDelay = time.Minute * 5

	coordinatorName = "locksmithd"
)

var (
	// TODO(mischief): daemon is not really a seperate package. it probably should be.
	dlog = capnslog.NewPackageLogger("github.com/coreos/locksmith", "daemon")
)

const (
	// The following constants represent the three strategies locksmith can take
	// for the checking if it is okay for the machine to reboot.

	// StrategyReboot reboots the machine as soon as it is instructed to do so,
	// without taking a lock.
	StrategyReboot = "reboot"

	// StrategyEtcdLock connects to the configured etcd and stores the lock in
	// there. Before it reboots, it aquires the lock, and will not reboot until
	// it does.
	StrategyEtcdLock = "etcd-lock"

	// StrategyOff causes locksmith to exit without performing any actions
	StrategyOff = "off"
)

// attempt to broadcast msg to all lines registered in utmp
// returns count of lines successfully opened (and likely broadcasted to)
func broadcast(msg string) uint {
	var cnt uint

	// move the utmp file pointer to the beginning of the utmp file
	C.setutent()

	// loop until we're out of utmp lines
	for {
		// read another line from the utmp file
		var utmp *C.struct_utmp
		utmp = C.getutent()
		if utmp == nil {
			// if no struct was returned, then there are no more lines and we're
			// done
			break
		}

		// get the device name of this user's tty out of the utmp struct
		line := C.GoString(&utmp.ut_line[0])
		tty, _ := os.OpenFile("/dev/"+line, os.O_WRONLY, 0)
		if tty == nil {
			// if we couldn't open the tty for this user, skip this user
			continue
		}
		// increment the counter of lines successfully opened
		cnt++
		// attempt to (asynchronously) write to the tty
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

func (r rebooter) rebootAndSleep() {
	// Broadcast a notice, if broadcast found lines to notify, delay the reboot.
	delaymins := loginsRebootDelay / time.Minute
	lines := broadcast(fmt.Sprintf("System reboot in %d minutes!", delaymins))
	if 0 != lines {
		dlog.Noticef("Logins detected, delaying reboot for %d minutes.", delaymins)
		time.Sleep(loginsRebootDelay)
	}
	r.lgn.Reboot(false)
	dlog.Info("Reboot sent. Going to sleep.")
	if err := r.coordinatorConfigUpdater.UpdateState(coordinatorconf.CoordinatorStateRebooting); err != nil {
		dlog.Errorf("could not update state file to indicate rebooting: %v", err)
	}

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

		r.rebootAndSleep()
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

type rebooter struct {
	strategy                 string
	lgn                      *login1.Conn
	coordinatorConfigUpdater coordinatorconf.CoordinatorConfigUpdater
}

func (r rebooter) reboot() int {
	if err := r.coordinatorConfigUpdater.UpdateState(coordinatorconf.CoordinatorStateRebootPlanned); err != nil {
		dlog.Errorf("could not update state file to indicate reboot planned: %v", err)
	}

	switch r.strategy {
	case StrategyEtcdLock:
		// If the strategy is etcd-lock, then a lock should be acquired in etcd
		// before rebooting
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
	case StrategyReboot:
		// If the strategy is reboot, no extra work must be done before
		// rebooting
	case StrategyOff:
		// We should never get here with the off strategy, but in case we do
		// print a more descriptive error message
		dlog.Error("can't reboot for strategy 'off'")
		return 1
	default:
		dlog.Errorf("unknown strategy: %s", r.strategy)
		return 1
	}

	r.rebootAndSleep()
	dlog.Fatal("Tried to reboot but did not!")
	return 1
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

// unlockHeldLocks will loop until it can confirm that any held locks are
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
		strategy = StrategyReboot
	}

	if strategy == StrategyOff {
		dlog.Noticef("Reboot strategy is %q - locksmithd is exiting.", strategy)
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

	coordinatorConf, err := coordinatorconf.New(coordinatorName, strategy)
	if err != nil {
		dlog.Fatalf("unable to become 'update coordinator': %v", err)
	}

	shutdown := make(chan os.Signal, 1)
	stop := make(chan struct{}, 1)

	go func() {
		<-shutdown
		dlog.Notice("Received interrupt/termination signal - locksmithd is exiting.")
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
	if strategy == StrategyEtcdLock {
		wg.Add(1)
		go unlockHeldLocks(strategy, stop, &wg)
	}

	ch := make(chan updateengine.Status, 1)
	go ue.RebootNeededSignal(ch, stop)

	r := rebooter{
		strategy: strategy,
		lgn:      lgn,
		coordinatorConfigUpdater: coordinatorConf,
	}

	result, err := ue.GetStatus()
	if err != nil {
		dlog.Fatalf("Cannot get update engine status: %v", err)
	}

	dlog.Infof("locksmithd starting currentOperation=%q strategy=%q", result.CurrentOperation, strategy)
	if err := r.coordinatorConfigUpdater.UpdateState(coordinatorconf.CoordinatorStateRunning); err != nil {
		dlog.Errorf("could not indicate 'running' in state file: %v", err)
	}

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
