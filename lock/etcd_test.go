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
	"errors"
	"reflect"
	"testing"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

type testEtcdClient struct {
	err  error
	resp *client.Response
}

func (t *testEtcdClient) Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error) {
	return t.resp, t.err
}

func (t *testEtcdClient) Set(ctx context.Context, key, value string, opts *client.SetOptions) (*client.Response, error) {
	return t.resp, t.err
}

func (t *testEtcdClient) Create(ctx context.Context, key, value string) (*client.Response, error) {
	return t.resp, t.err
}

func TestEtcdLockClientInit(t *testing.T) {
	for i, tt := range []struct {
		ee      error
		want    bool
		group   string
		keypath string
	}{
		{nil, false, "", SemaphorePrefix},
		{client.Error{Code: client.ErrorCodeNodeExist}, false, "", SemaphorePrefix},
		{client.Error{Code: client.ErrorCodeKeyNotFound}, true, "", SemaphorePrefix},
		{errors.New("some random error"), true, "", SemaphorePrefix},
		{client.Error{Code: client.ErrorCodeKeyNotFound}, true, "database", "coreos.com/updateengine/rebootlock/groups/database/semaphore"},
		{nil, false, "prod/database", "coreos.com/updateengine/rebootlock/groups/prod%2Fdatabase/semaphore"},
	} {
		elc, got := NewEtcdLockClient(&testEtcdClient{err: tt.ee}, tt.group)
		if (got != nil) != tt.want {
			t.Errorf("case %d: unexpected error state initializing Client: got %v", i, got)
			continue
		}

		if got != nil {
			continue
		}

		if elc.keypath != tt.keypath {
			t.Errorf("case %d: unexpected etcd key path: got %v want %v", i, elc.keypath, tt.keypath)
		}
	}
}

func makeResponse(idx int, val string) *client.Response {
	return &client.Response{
		Node: &client.Node{
			Value:         val,
			ModifiedIndex: uint64(idx),
		},
	}
}

func TestEtcdLockClientGet(t *testing.T) {
	for i, tt := range []struct {
		ee error
		er *client.Response
		ws *Semaphore
		we bool
	}{
		// errors returned from etcd
		{errors.New("some error"), nil, nil, true},
		{client.Error{Code: client.ErrorCodeKeyNotFound}, nil, nil, true},
		// bad JSON should cause errors
		{nil, makeResponse(0, "asdf"), nil, true},
		{nil, makeResponse(0, `{"semaphore:`), nil, true},
		// successful calls
		{nil, makeResponse(10, `{"semaphore": 1}`), &Semaphore{Index: 10, Semaphore: 1}, false},
		{nil, makeResponse(1024, `{"semaphore": 1, "max": 2, "holders": ["foo", "bar"]}`), &Semaphore{Index: 1024, Semaphore: 1, Max: 2, Holders: []string{"foo", "bar"}}, false},
		// index should be set from etcd, not json!
		{nil, makeResponse(1234, `{"semaphore": 89, "index": 4567}`), &Semaphore{Index: 1234, Semaphore: 89}, false},
	} {
		elc := &EtcdLockClient{
			keyapi: &testEtcdClient{
				err:  tt.ee,
				resp: tt.er,
			},
		}
		gs, ge := elc.Get()
		if tt.we {
			if ge == nil {
				t.Fatalf("case %d: expected error but got nil!", i)
			}
		} else {
			if ge != nil {
				t.Fatalf("case %d: unexpected error: %v", i, ge)
			}
		}
		if !reflect.DeepEqual(gs, tt.ws) {
			t.Fatalf("case %d: bad semaphore: got %#v, want %#v", i, gs, tt.ws)
		}
	}
}

func TestEtcdLockClientSet(t *testing.T) {
	for i, tt := range []struct {
		sem  *Semaphore
		ee   error // error returned from etcd
		want bool  // do we expect Set to return an error
	}{
		// nil semaphore cannot be set
		{nil, nil, true},
		// empty semaphore is OK
		{&Semaphore{}, nil, false},
		{&Semaphore{Index: uint64(1234)}, nil, false},
		// all errors returned from etcd should propagate
		{&Semaphore{}, client.Error{Code: client.ErrorCodeNodeExist}, true},
		{&Semaphore{}, client.Error{Code: client.ErrorCodeKeyNotFound}, true},
		{&Semaphore{}, errors.New("some random error"), true},
	} {
		elc := &EtcdLockClient{
			keyapi: &testEtcdClient{err: tt.ee},
		}
		got := elc.Set(tt.sem)
		if (got != nil) != tt.want {
			t.Errorf("case %d: unexpected error state calling Set: got %v", i, got)
		}
	}
}
