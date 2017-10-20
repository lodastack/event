package cluster

import (
	"time"

	"github.com/coreos/etcd/client"
)

// Inf is the interface of etcd have.
type Inf interface {
	// Get return k-v from etcd.
	Get(k string, option *client.GetOptions) (*client.Response, error)

	// Set k-v to etcd with option.
	Set(k, v string, option *client.SetOptions) error

	// SetWithTTL set k-v to etcd with TTL.
	SetWithTTL(k, v string, duration time.Duration) error

	// Remove k-v from etcd.
	Remove(key string) error

	//RemoveDir remove directory from etcd.
	RemoveDir(k string) error

	// RecursiveGet return k and its child k.
	RecursiveGet(k string) (*client.Response, error)

	// Mkdir make a new directory to etcd.
	Mkdir(k string) error
}

// NewCluster return cluster interface.
func NewCluster(selfAddr string, endpoints []string, basicAuth bool, username, password string,
	headTimeout, nodeTTL time.Duration) (Inf, error) {
	var c client.Client
	var kapi client.KeysAPI
	var err error

	cfg := client.Config{
		Endpoints: endpoints,
	}
	if headTimeout != 0 {
		cfg.HeaderTimeoutPerRequest = headTimeout * time.Second
	}
	if basicAuth {
		cfg.Username = username
		cfg.Password = password
	}

	c, err = client.New(cfg)
	if err != nil {
		return nil, err
	}
	if nodeTTL == 0 {
		nodeTTL = defaultNodeTTL
	}
	kapi = client.NewKeysAPI(c)
	return &etcdClient{kapi: kapi,
		EndPoints: endpoints}, nil
}
