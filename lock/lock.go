package lock

type Lock struct {
	id string
	client LockClient
}

func New(id string, client LockClient) (lock *Lock) {
	return &Lock{id, client}
}

func (l *Lock) store(f func(*semaphore) error) (err error) {
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

func (l *Lock) SetMax(max int) (err error) {
	return l.store(func(sem *semaphore) error {
		return sem.SetMax(max)
	})
}

func (l *Lock) Lock() (err error) {
	return l.store(func(sem *semaphore) error {
		return sem.Lock(l.id)
	})
}

func (l *Lock) Unlock() error {
	return l.store(func(sem *semaphore) error {
		return sem.Unlock(l.id)
	})
}
