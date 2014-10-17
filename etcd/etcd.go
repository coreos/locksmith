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
