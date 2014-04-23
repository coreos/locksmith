package lock

import (
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
	if c.sem.Holders[0] != "a" {
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

	if c.sem.Holders[0] != "a" || len(c.sem.Holders) > 1 {
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

func TestDoubleLockSuccess(t *testing.T) {
	c := testLockClient{}
	c.Init()
	al := New("a", &c)
	bl := New("b", &c)

	al.SetMax(2)

	err := al.Lock()
	if err != nil {
		t.Error(err)
	}

	err = bl.Lock()
	if err != nil {
		t.Error(err)
	}

	if c.sem.Holders[1] != "b" || c.sem.Holders[0] != "a" || len(c.sem.Holders) != 2 {
		t.Error("Lock did not add a to the holders")
	}

	if c.sem.Semaphore != 0 {
		t.Error("Lock did not decrement the semaphore")
	}

	al.Unlock()
	if len(c.sem.Holders) != 1 {
		t.Error("Lock did not remove a from the holders")
	}

	if c.sem.Semaphore != 1 {
		t.Error("Lock did not increment the semaphore")
	}

	// TODO(philips): make setmax it's own test
	for i := range []int{3, 2, 1, 0, -1, 0, 1, 2, 3} {
		al.SetMax(i)
		if c.sem.Semaphore != i-1 {
			t.Error("SetMax did not increment the semaphore")
		}
	}

	al.SetMax(0)

	err = bl.Unlock()
	if err != nil {
		t.Error(err)
	}

	if c.sem.Semaphore != 0 {
		t.Error("Sempahore not at 0")
	}
}
