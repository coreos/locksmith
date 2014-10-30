/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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

	"github.com/coreos/locksmith/lock"
	"github.com/coreos/locksmith/pkg/machineid"
	"github.com/coreos/locksmith/updateengine"
)

const (
	initialInterval   = time.Second * 5
	maxInterval       = time.Minute * 5
	loginsRebootDelay = time.Minute * 5
)

const (
	StrategyReboot     = "reboot"
	StrategyEtcdLock   = "etcd-lock"
	StrategyBestEffort = "best-effort"
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
		fmt.Printf("Logins detected, delaying reboot for %d minutes.\n", delaymins)
		time.Sleep(loginsRebootDelay)
	}
	lgn.Reboot(false)
	fmt.Println("Reboot sent. Going to sleep.")

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
			fmt.Printf("Retrying in %v. Error locking: %v\n", interval, err)
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
func etcdActive() (running bool, err error) {
	sys, err := dbus.New()
	if err != nil {
		return false, err
	}
	defer sys.Close()

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
	lgn      *login1.Conn
}

func (r rebooter) useLock() (useLock bool, err error) {
	switch r.strategy {
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

		err = unlockIfHeld(lck)
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

// unlockIfHeld will unlock a lock, if it is held by this machine, or return an error.
func unlockIfHeld(lck *lock.Lock) error {
	err := lck.Unlock()
	if err == lock.ErrNotExist {
		return nil
	} else if err == nil {
		fmt.Println("Unlocked existing lock for this machine")
		return nil
	}

	return err
}

// unlockHeldLock will loop until it can confirm that any held locks are
// released or a stop signal is sent.
func unlockHeldLocks(stop chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	interval := initialInterval
	for {
		var reason string
		select {
		case <-stop:
			return
		case <-time.After(interval):
			active, err := etcdActive()
			if err != nil {
				reason = "error checking on etcd.service"
				break
			}
			if !active {
				reason = "etcd.service is inactive"
				break
			}

			lck, err := setupLock()
			if err != nil {
				reason = "error setting up lock"
				break
			}

			err = unlockIfHeld(lck)
			if err == nil {
				return
			}
			reason = err.Error()
		}

		interval = expBackoff(interval)
		fmt.Printf("Unlocking old locks failed: %v. Retrying in %v.\n", reason, interval)
	}
}

// runDaemon waits for the reboot needed signal coming out of update engine and
// attempts to acquire the reboot lock. If the reboot lock is acquired then the
// machine will reboot.
func runDaemon() int {
	shutdown := make(chan os.Signal, 1)
	stop := make(chan struct{}, 1)
	go func() {
		<-shutdown
		fmt.Fprintln(os.Stderr, "Received interrupt/termination signal - shutting down.")
		os.Exit(0)
	}()
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	strategy := os.ExpandEnv("${REBOOT_STRATEGY}")
	if strategy == "" {
		strategy = StrategyBestEffort
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

	var wg sync.WaitGroup
	if strategy != StrategyReboot {
		wg.Add(1)
		go unlockHeldLocks(stop, &wg)
	}

	ch := make(chan updateengine.Status, 1)
	go ue.RebootNeededSignal(ch, stop)

	r := rebooter{strategy, lgn}

	result, err := ue.GetStatus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot get update engine status:", err)
		return 1
	}

	fmt.Printf("locksmithd starting currentOperation=%q strategy=%q\n",
		result.CurrentOperation,
		strategy,
	)

	if result.CurrentOperation != updateengine.UpdateStatusUpdatedNeedReboot {
		<-ch
	}

	close(stop)
	wg.Wait()

	return r.reboot()
}
