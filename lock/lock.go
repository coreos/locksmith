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

package lock

// Lock takes care of locking in generic clients
type Lock struct {
	id     string
	client LockClient
}

// New returns a new lock with the provided arguments
func New(id string, client LockClient) (lock *Lock) {
	return &Lock{id, client}
}

func (l *Lock) store(f func(*Semaphore) error) (err error) {
	sem, err := l.client.Get()
	if err != nil {
		return err
	}

	if err := f(sem); err != nil {
		return err
	}

	err = l.client.Set(sem)
	if err != nil {
		return err
	}

	return nil
}

// Get returns the current semaphore value
// if the underlying client returns an error, Get passes it through
func (l *Lock) Get() (sem *Semaphore, err error) {
	sem, err = l.client.Get()
	if err != nil {
		return nil, err
	}

	return sem, nil
}

// SetMax sets the maximum number of holders the semaphore will allow
// it returns the current semaphore and the previous maximum
// if there is a problem getting or setting the semaphore, this function will
// pass on errors from the underlying client
func (l *Lock) SetMax(max int) (sem *Semaphore, oldMax int, err error) {
	var (
		semRet *Semaphore
		old    int
	)

	return semRet, old, l.store(func(sem *Semaphore) error {
		old = sem.Max
		semRet = sem
		return sem.SetMax(max)
	})
}

// Lock adds this lock id as a holder to the semaphore
// it will return an error if there is a problem getting or setting the
// semaphore, or if the maximum number of holders has been reached, or if a lock
// with this id is already a holder
func (l *Lock) Lock() (err error) {
	return l.store(func(sem *Semaphore) error {
		return sem.Lock(l.id)
	})
}

// Unlock removes this lock id as a holder of the semaphore
// it returns an error if there is a problem getting or setting the semaphore,
// or if this lock is not locked.
func (l *Lock) Unlock() error {
	return l.store(func(sem *Semaphore) error {
		return sem.Unlock(l.id)
	})
}
