package lock

import (
	"encoding/json"
	"errors"

	// TODO(jonboulle): this is a leaky abstraction, but we don't want to reimplement all go-etcd types in locksmith/etcd. This should go away once go-etcd is replaced.
	goetcd "github.com/coreos/locksmith/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/locksmith/etcd"
)

const (
	keyPrefix       = "coreos.com/updateengine/rebootlock"
	holdersPrefix   = keyPrefix + "/holders"
	SemaphorePrefix = keyPrefix + "/semaphore"
)

// EtcdLockClient is a wrapper around the etcd client that provides
// simple primitives to operate on the internal semaphore and holders
// structs through etcd.
type EtcdLockClient struct {
	client etcd.EtcdClient
}

func NewEtcdLockClient(ec etcd.EtcdClient) (client *EtcdLockClient, err error) {
	client = &EtcdLockClient{ec}
	err = client.Init()
	return
}

// Init sets an initial copy of the semaphore if it doesn't exist yet.
func (c *EtcdLockClient) Init() (err error) {
	sem := newSemaphore()
	b, err := json.Marshal(sem)
	if err != nil {
		return err
	}

	_, err = c.client.Create(SemaphorePrefix, string(b), 0)
	if err != nil {
		eerr, ok := err.(*goetcd.EtcdError)
		if ok && eerr.ErrorCode == etcd.ErrorNodeExist {
			return nil
		}
	}

	return err
}

// Get fetches the Semaphore from etcd.
func (c *EtcdLockClient) Get() (sem *Semaphore, err error) {
	resp, err := c.client.Get(SemaphorePrefix, false, false)
	if err != nil {
		return nil, err
	}

	sem = &Semaphore{}
	err = json.Unmarshal([]byte(resp.Node.Value), sem)
	if err != nil {
		return nil, err
	}

	sem.Index = resp.Node.ModifiedIndex

	return sem, nil
}

// Set sets a Semaphore in etcd.
func (c *EtcdLockClient) Set(sem *Semaphore) (err error) {
	if sem == nil {
		return errors.New("cannot set nil semaphore")
	}
	b, err := json.Marshal(sem)
	if err != nil {
		return err
	}

	_, err = c.client.CompareAndSwap(SemaphorePrefix, string(b), 0, "", sem.Index)

	return err
}
