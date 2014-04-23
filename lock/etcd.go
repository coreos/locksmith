package lock

import (
	"encoding/json"

	etcdError "github.com/coreos/etcd/error"
	"github.com/coreos/go-etcd/etcd"
)

const (
	keyPrefix = "coreos.com/updateengine/rebootlock"
	holdersPrefix = keyPrefix + "/holders"
	semaphorePrefix = keyPrefix + "/sempahore"
)

// EtcdLockClient is a wrapper around the go-etcd client that provides
// simple primitives to operate on the internal sempahore and holders
// structs through etcd.
type EtcdLockClient struct {
	client *etcd.Client
}

func NewEtcdLockClient(machines []string) (client *EtcdLockClient, err error) {
	ec := etcd.NewClient(machines)
	client = &EtcdLockClient{ec}
	err = client.Init()

	return client, err
}

// Init sets an initial copy of the sempahore if it doesn't exist yet.
func (c *EtcdLockClient) Init() (err error) {
	sem := newSemaphore()
	b, err := json.Marshal(sem)
	if err != nil {
		return err
	}

	// TODO(philips): 
	_, err = c.client.Create(semaphorePrefix, string(b), 0)
	if err != nil {
		eerr, ok := err.(*etcd.EtcdError)
		if ok && eerr.ErrorCode == etcdError.EcodeNodeExist {
			return nil
		}
	}

	return err
}

// Get fetches the semaphore from etcd.
func (c *EtcdLockClient) Get() (sem *semaphore, err error) {
	resp, err := c.client.Get(semaphorePrefix, false, false)
	if err != nil {
		return nil, err
	}

	sem = &semaphore{}
	err = json.Unmarshal([]byte(resp.Node.Value), sem)
	if err != nil {
		return nil, err
	}

	sem.Index = resp.Node.ModifiedIndex

	return sem, nil
}

// Set sets a semaphore in etcd.
func (c *EtcdLockClient) Set(sem *semaphore) (err error) {
	b, err := json.Marshal(sem)
	if err != nil {
		return err
	}

	_, err = c.client.CompareAndSwap(semaphorePrefix, string(b), 0, "", sem.Index)

	return nil
}
