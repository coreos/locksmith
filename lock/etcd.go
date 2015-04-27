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
	"encoding/json"
	"errors"
	"net/url"
	"path"

	// TODO(jonboulle): this is a leaky abstraction, but we don't want to reimplement all go-etcd types in locksmith/etcd. This should go away once go-etcd is replaced.
	goetcd "github.com/coreos/locksmith/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/locksmith/etcd"
)

const (
	keyPrefix       = "coreos.com/updateengine/rebootlock"
	groupBranch     = "groups"
	semaphoreBranch = "semaphore"
	SemaphorePrefix = keyPrefix + "/" + semaphoreBranch
)

// EtcdLockClient is a wrapper around the etcd client that provides
// simple primitives to operate on the internal semaphore and holders
// structs through etcd.
type EtcdLockClient struct {
	client  etcd.EtcdClient
	keypath string
}

// NewEtcdLockClient creates a new EtcdLockClient. The group parameter defines
// the etcd key path in which the client will manipulate the semaphore. If the
// group is the empty string, the default semaphore will be used.
func NewEtcdLockClient(ec etcd.EtcdClient, group string) (client *EtcdLockClient, err error) {
	key := SemaphorePrefix
	if group != "" {
		key = path.Join(keyPrefix, groupBranch, url.QueryEscape(group), semaphoreBranch)
	}

	client = &EtcdLockClient{ec, key}
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

	_, err = c.client.Create(c.keypath, string(b), 0)
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
	resp, err := c.client.Get(c.keypath, false, false)
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

	_, err = c.client.CompareAndSwap(c.keypath, string(b), 0, "", sem.Index)

	return err
}
