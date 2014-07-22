package lock

import (
	"errors"
	"reflect"
	"testing"

	etcdError "github.com/coreos/locksmith/third_party/github.com/coreos/etcd/error"
	"github.com/coreos/locksmith/third_party/github.com/coreos/go-etcd/etcd"
)

type testEtcdClient struct {
	err  error
	resp *etcd.Response
}

func (t *testEtcdClient) Create(key string, value string, ttl uint64) (*etcd.Response, error) {
	return t.resp, t.err
}

func (t *testEtcdClient) CompareAndSwap(key string, value string, ttl uint64, prevValue string, prevIndex uint64) (*etcd.Response, error) {
	return t.resp, t.err
}

func (t *testEtcdClient) Get(key string, sort, recursive bool) (*etcd.Response, error) {
	return t.resp, t.err
}

func TestEtcdLockClientInit(t *testing.T) {
	for i, tt := range []struct {
		ee   error
		want bool
	}{
		{nil, false},
		{&etcd.EtcdError{ErrorCode: etcdError.EcodeNodeExist}, false},
		{&etcd.EtcdError{ErrorCode: etcdError.EcodeKeyNotFound}, true},
		{errors.New("some random error"), true},
	} {
		elc := &EtcdLockClient{
			client: &testEtcdClient{err: tt.ee},
		}
		got := elc.Init()
		if (got != nil) != tt.want {
			t.Errorf("case %d: unexpected error state initializing Client: got %v", i, got)
		}
	}
}

func TestEtcdLockClientGet(t *testing.T) {
	for i, tt := range []struct {
		ee error
		er *etcd.Response
		ws *Semaphore
		we error
	}{
		{errors.New("some error"), nil, nil, errors.New("some error")},
		// TODO(jonboulle): add real tests here
	} {
		elc := &EtcdLockClient{
			client: &testEtcdClient{
				err:  tt.ee,
				resp: tt.er,
			},
		}
		gs, ge := elc.Get()
		if tt.ee != nil {
			if ge == nil {
				t.Fatalf("case %d: expected error but got nil!", i)
			}
		} else {
			if !reflect.DeepEqual(gs, tt.ws) {
				t.Fatalf("case %d: bad semaphore: got %#v, want %#v", gs, tt.ws)
			}

		}
	}
}

func TestEtcdLockClientSet(t *testing.T) {
	for i, tt := range []struct {
		ee   error
		want bool
	}{
		{nil, false},
		{&etcd.EtcdError{ErrorCode: etcdError.EcodeNodeExist}, true},
		{&etcd.EtcdError{ErrorCode: etcdError.EcodeKeyNotFound}, true},
		{errors.New("some random error"), true},
	} {
		elc := &EtcdLockClient{
			client: &testEtcdClient{err: tt.ee},
		}
		got := elc.Set(&Semaphore{})
		if (got != nil) != tt.want {
			t.Errorf("case %d: unexpected error state calling Set: got %v", i, got)
		}
	}
}
