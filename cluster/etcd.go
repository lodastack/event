package cluster

import (
	"context"
	"strings"
	"time"

	"github.com/lodastack/event/config"

	"github.com/coreos/etcd/client"
)

var (
	defaultNodeTTL time.Duration = 5
)

// EtcdClient is imformation the etcd client have.
type etcdClient struct {
	kapi client.KeysAPI

	// etcd endpoints.
	EndPoints []string
}

// Get return value of a key.
func (c *etcdClient) Get(k string, option *client.GetOptions) (*client.Response, error) {
	key := config.GetConfig().Etcd.Path + "/" + k
	return c.kapi.Get(context.Background(), key, option)
}

// RecursiveGet return etcd/client.Response contain the node and its child nodes.
func (c *etcdClient) RecursiveGet(k string) (*client.Response, error) {
	if !strings.HasPrefix(k, config.GetConfig().Etcd.Path) {
		k = config.GetConfig().Etcd.Path + "/" + k
	}

	return c.kapi.Get(context.Background(), k, &client.GetOptions{Recursive: true})
}

// Set set a k/v to etcd with the SetOptions.
func (c *etcdClient) Set(k, v string, option *client.SetOptions) error {
	key := config.GetConfig().Etcd.Path + "/" + k
	_, err := c.kapi.Set(context.Background(), key, v, option)
	return err
}

// SetWithTTL set a k-v with TTL.
func (c *etcdClient) SetWithTTL(k, v string, duration time.Duration) error {
	key := config.GetConfig().Etcd.Path + "/" + k
	if duration == 0 {
		duration = 10 * time.Minute
	}
	_, err := c.kapi.Set(context.Background(), key, v, &client.SetOptions{TTL: duration})
	return err
}

// Remove remove a key from etcd.
func (c *etcdClient) Remove(k string) error {
	key := k // NOTE: not add prefix
	_, err := c.kapi.Delete(context.Background(), key, nil)
	return err
}

// RemoveDir remove a dir from etcd.
func (c *etcdClient) RemoveDir(k string) error {
	key := k // NOTE: not add prefix
	_, err := c.kapi.Delete(context.Background(), key, &client.DeleteOptions{Dir: true, Recursive: true})
	return err
}

// Mkdir make a dir to etcd.
func (c *etcdClient) Mkdir(k string) error {
	return c.Set(k, k, &client.SetOptions{Dir: true})
}
