// Copyright 2017 CoreOS, Inc.
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

// Package filelock implements an exclusive filesystem lock which allows the
// caller to atomically update the file safely as well.
// It is built on top of the rkt `filelock` package.
// It is partially related to discussion in https://github.com/rkt/rkt/pull/3615
package filelock

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/rkt/rkt/pkg/lock"
)

var AlreadyUnlockedErr = errors.New("lock is already unlocked")

// UpdateableFileLock is a filelock which can be updated atomically
type UpdateableFileLock struct {
	// lockLock protects updating of the file lock. It ensures that `unlock` will
	// correctly unlock the most up to date file lock
	lockLock sync.Mutex
	lock     *lock.FileLock
	isLocked bool
	path     string
	perms    os.FileMode
}

// NewExclusiveLock creates a new exclusive filelock.
// The given filepath must exist.
func NewExclusiveLock(path string) (*UpdateableFileLock, error) {
	flock, err := lock.TryExclusiveLock(path, lock.RegFile)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	// Note: if we expected the file to ever be deleted and recreated while an
	// ExLock isn't held, we would have to check the fd is still correct here.
	// However, the file is created once and never deleted outside of the safely
	// handled renames in this package, so it's safe to assume the fd hasn't
	// changed.
	// The rename call in this package is "safe" because it leaves no window
	// where the file at that path does not have an exclusive lock
	return &UpdateableFileLock{
		lock:     flock,
		path:     path,
		isLocked: true,
		perms:    fi.Mode(),
	}, nil
}

// Update writes the given contents into the filelock atomically
func (l *UpdateableFileLock) Update(contents io.Reader) error {
	lockDir := filepath.Dir(l.path)
	newFile, err := ioutil.TempFile(lockDir, fmt.Sprintf(".%s", filepath.Base(l.path)))
	if err != nil {
		return err
	}
	defer newFile.Close()
	if err := newFile.Chmod(l.perms); err != nil {
		os.Remove(newFile.Name())
		return err
	}

	_, err = io.Copy(newFile, contents)
	if err != nil {
		os.Remove(newFile.Name())
		return err
	}

	newFileLock, err := lock.TryExclusiveLock(newFile.Name(), lock.RegFile)
	if err != nil {
		os.Remove(newFile.Name())
		return fmt.Errorf("could not lock tmpfile: %v", err)
	}

	l.lockLock.Lock()
	defer l.lockLock.Unlock()

	if !l.isLocked {
		os.Remove(newFile.Name())
		return AlreadyUnlockedErr
	}
	// Overwrite the old lock with our new one that has the correct contents
	err = os.Rename(newFile.Name(), l.path)
	if err != nil {
		return err
	}
	// Lock overwritten, update our internal state while we still hold the mutex
	oldLock := l.lock
	l.lock = newFileLock
	oldLock.Unlock()
	return nil
}

// Unlock unlocks the filelock
func (l *UpdateableFileLock) Unlock() error {
	l.lockLock.Lock()
	defer l.lockLock.Unlock()

	if !l.isLocked {
		return AlreadyUnlockedErr
	}
	l.isLocked = false
	return l.lock.Unlock()
}
