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

package etcd

import "github.com/coreos/locksmith/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"

// EtcdClient is a simple wrapper around the underlying etcd client to facilitate
// testing and minimize the interface between locksmith and the actual etcd client
type EtcdClient interface {
	Create(key string, value string, ttl uint64) (*etcd.Response, error)
	CompareAndSwap(key string, value string, ttl uint64, prevValue string, prevIndex uint64) (*etcd.Response, error)
	Get(key string, sort, recursive bool) (*etcd.Response, error)
}

type TLSInfo struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

const (
	ErrorKeyNotFound = 100
	ErrorNodeExist   = 105
)

// NewClient creates a new EtcdClient
func NewClient(machines []string, ti *TLSInfo) (EtcdClient, error) {
	if ti != nil {
		return etcd.NewTLSClient(machines, ti.CertFile, ti.KeyFile, ti.CAFile)
	}
	return etcd.NewClient(machines), nil
}
