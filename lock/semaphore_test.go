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

import (
	"reflect"
	"testing"
)

type testLockClient struct {
	sem     *Semaphore
	holders []holder
}

func (c *testLockClient) Init() (err error) {
	c.sem = newSemaphore()
	return nil
}

func (c *testLockClient) Get() (sem *Semaphore, err error) {
	return c.sem, nil
}

func (c *testLockClient) Set(sem *Semaphore) (err error) {
	c.sem = sem
	return nil
}

func TestTestLockClient(t *testing.T) {
	c := testLockClient{}
	c.Init()
	sem, _ := c.Get()
	c.Set(sem)
}

func TestSingleLock(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)

	al.Lock()
	if !reflect.DeepEqual(c.sem.Holders, []string{"a"}) {
		t.Error("Lock did not add a to the holders")
	}

	if c.sem.Semaphore != 0 {
		t.Error("Lock did not decrement the semaphore")
	}

	al.Unlock()
	if len(c.sem.Holders) != 0 {
		t.Error("Lock did not remove a from the holders")
	}

	if c.sem.Semaphore != 1 {
		t.Error("Lock did not increment the semaphore")
	}
}

func TestSingleDeadlock(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)

	if err := al.Lock(); err != nil {
		t.Error(err)
	}

	if err := al.Lock(); err == nil {
		t.Error(err)
	}

	if err := al.Unlock(); err != nil {
		t.Error(err)
	}
}

func TestSameDoubleLockFail(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)
	al.SetMax(2)
	err := al.Lock()
	if err != nil {
		t.Fatal(err)
	}
	err = al.Lock()
	if err == nil {
		t.Error("Same holder locking twice should have failed")
	}
}

func TestUnlockUnheldLockFail(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)
	if err := al.Unlock(); err == nil {
		t.Error("Unlocking lock with zero holders should have failed", err)
	}

	if err := al.Lock(); err != nil {
		t.Fatal(err)
	}

	bl := New("b", &c)
	if err := bl.Unlock(); err == nil {
		t.Error("Unlocking unheld lock should have failed", err)
	}
}

func TestDoubleLockFail(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)
	bl := New("b", &c)

	err := al.Lock()
	if err != nil {
		t.Error(err)
	}
	err = bl.Lock()
	if err == nil {
		t.Error("Second lock should have failed")
	}

	if !reflect.DeepEqual(c.sem.Holders, []string{"a"}) {
		t.Error("Lock did not add a to the holders")
	}

	if c.sem.Semaphore != 0 {
		t.Error("Lock did not decrement the semaphore")
	}

	al.Unlock()
	if len(c.sem.Holders) != 0 {
		t.Error("Unlock did not remove a from the holders")
	}

	if c.sem.Semaphore != 1 {
		t.Error("Unlock did not increment the semaphore")
	}
}

func TestDoubleLockSuccess(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)
	bl := New("b", &c)

	al.SetMax(2)

	err := al.Lock()
	if err != nil {
		t.Fatal(err)
	}

	err = bl.Lock()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(c.sem.Holders, []string{"a", "b"}) {
		t.Error("Lock did not add b to the holders")
	}

	if c.sem.Semaphore != 0 {
		t.Error("Lock did not decrement the semaphore")
	}

	al.Unlock()
	if !reflect.DeepEqual(c.sem.Holders, []string{"b"}) {
		t.Error("Unlock did not remove a from the holders")
	}

	if c.sem.Semaphore != 1 {
		t.Error("Unlock did not increment the semaphore")
	}

}

func TestHolderOrdering(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)
	bl := New("b", &c)
	cl := New("c", &c)

	al.SetMax(3)

	cl.Lock()
	bl.Lock()
	if !reflect.DeepEqual(c.sem.Holders, []string{"b", "c"}) {
		t.Error("initial ordering failed", c.sem.Holders)
	}
	al.Lock()
	if !reflect.DeepEqual(c.sem.Holders, []string{"a", "b", "c"}) {
		t.Error("inserting a broke expected ordering")
	}
	bl.Unlock()
	if !reflect.DeepEqual(c.sem.Holders, []string{"a", "c"}) {
		t.Error("removing b broke expected ordering")
	}
	cl.Unlock()
	if !reflect.DeepEqual(c.sem.Holders, []string{"a"}) {
		t.Error("removing c broke expected ordering")
	}
	bl.Lock()
	if !reflect.DeepEqual(c.sem.Holders, []string{"a", "b"}) {
		t.Error("adding b broke expected ordering")
	}
}

func TestSetMax(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)
	al.Lock()
	for i := range []int{3, 2, 1, 0, -1, 0, 1, 2, 3} {
		al.SetMax(i)
		if c.sem.Semaphore != i-1 {
			t.Error("SetMax did not increment the semaphore", c.sem.Semaphore)
		}
	}
}
