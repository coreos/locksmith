package lock

type LockClient interface {
	Init() (*semaphore, error)
	Get() (*semaphore, error)
	Set(*semaphore) (error)
}
