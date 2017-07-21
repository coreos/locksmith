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

	"github.com/coreos/etcd/client"

	"golang.org/x/net/context"
)

const (
	keyPrefix       = "coreos.com/updateengine/rebootlock"
	groupBranch     = "groups"
	semaphoreBranch = "semaphore"
	// SemaphorePrefix is the key in etcd where the semaphore will be stored
	SemaphorePrefix = keyPrefix + "/" + semaphoreBranch
)

// KeysAPI is the minimum etcd client.KeysAPI interface EtcdLockClient needs
// to do its job.
type KeysAPI interface {
	Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error)
	Set(ctx context.Context, key, value string, opts *client.SetOptions) (*client.Response, error)
	Create(ctx context.Context, key, value string) (*client.Response, error)
}

// EtcdLockClient is a wrapper around the etcd client that provides
// simple primitives to operate on the internal semaphore and holders
// structs through etcd.
type EtcdLockClient struct {
	keyapi  KeysAPI
	keypath string
}

// NewEtcdLockClient creates a new EtcdLockClient. The group parameter defines
// the etcd key path in which the client will manipulate the semaphore. If the
// group is the empty string, the default semaphore will be used.
func NewEtcdLockClient(keyapi KeysAPI, group string) (*EtcdLockClient, error) {
	key := SemaphorePrefix
	if group != "" {
		key = path.Join(keyPrefix, groupBranch, url.QueryEscape(group), semaphoreBranch)
	}

	elc := &EtcdLockClient{keyapi, key}
	if err := elc.Init(); err != nil {
		return nil, err
	}

	return elc, nil
}

// Init sets an initial copy of the semaphore if it doesn't exist yet.
func (c *EtcdLockClient) Init() error {
	sem := newSemaphore()
	b, err := json.Marshal(sem)
	if err != nil {
		return err
	}

	if _, err := c.keyapi.Create(context.Background(), c.keypath, string(b)); err != nil {
		eerr, ok := err.(client.Error)
		if ok && eerr.Code == client.ErrorCodeNodeExist {
			return nil
		}

		return err
	}

	return nil
}

// Get fetches the Semaphore from etcd.
func (c *EtcdLockClient) Get() (*Semaphore, error) {
	resp, err := c.keyapi.Get(context.Background(), c.keypath, nil)
	if err != nil {
		return nil, err
	}

	sem := &Semaphore{}
	err = json.Unmarshal([]byte(resp.Node.Value), sem)
	if err != nil {
		return nil, err
	}

	sem.Index = resp.Node.ModifiedIndex

	return sem, nil
}

// Set sets a Semaphore in etcd.
func (c *EtcdLockClient) Set(sem *Semaphore) error {
	if sem == nil {
		return errors.New("cannot set nil semaphore")
	}
	b, err := json.Marshal(sem)
	if err != nil {
		return err
	}

	setopts := &client.SetOptions{
		PrevIndex: sem.Index,
	}

	_, err = c.keyapi.Set(context.Background(), c.keypath, string(b), setopts)
	return err
}
