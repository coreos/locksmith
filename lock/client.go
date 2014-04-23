package lock

type LockClient interface {
	Init() (error)
	Get() (*semaphore, error)
	Set(*semaphore) (error)
}
