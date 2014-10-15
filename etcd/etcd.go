package etcd

import (
	"fmt"

	"github.com/coreos/locksmith/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// EtcdClient is a simple wrapper around the go-etcd client to facilitate testing
type EtcdClient interface {
	Create(key string, value string, ttl uint64) (*etcd.Response, error)
	CompareAndSwap(key string, value string, ttl uint64, prevValue string, prevIndex uint64) (*etcd.Response, error)
	Get(key string, sort, recursive bool) (*etcd.Response, error)
}

type Error struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
	Cause     string `json:"cause"`
	Index     uint64 `json:"index"`
}

func (e Error) Error() string {
	return fmt.Sprintf("%v: %v (%v) [%v]", e.ErrorCode, e.Message, e.Cause, e.Index)
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
