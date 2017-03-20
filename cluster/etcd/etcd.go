package etcd

import (
	"fmt"
	"strings"
	"time"

	"github.com/lodastack/log"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

var (
	alarmsDirect                 = "loda-alarms"
	cacheCap                     = 10
	defaultNodeTTL time.Duration = 5

	Expire = "expire"
	Set    = "set"
	Delete = "delete"
)

type EtcdMsg struct {
	Value  string
	Action string
}

// Client is a wrapper around the etcd client
type EtcdClient struct {
	kapi              client.KeysAPI
	EndPoints         []string
	MsgChannle        chan EtcdMsg
	HeartBeatInterval time.Duration
}

func NewEtcdClient(endpoints []string, basicAuth bool, username, password string,
	headTimeout, nodeTTL time.Duration) (EtcdClient, error) {
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
		return EtcdClient{}, err
	}
	if nodeTTL == 0 {
		nodeTTL = defaultNodeTTL
	}
	kapi = client.NewKeysAPI(c)
	return EtcdClient{kapi: kapi,
		EndPoints:         endpoints,
		MsgChannle:        make(chan EtcdMsg, cacheCap),
		HeartBeatInterval: nodeTTL}, nil
}

func (c *EtcdClient) Watch(key string) (EtcdMsg, error) {
	watchKey := alarmsDirect
	if key != "" {
		watchKey = alarmsDirect + "/" + key
	}
	watcher := c.kapi.Watcher(watchKey, &client.WatcherOptions{
		Recursive: true,
	})

	res, err := watcher.Next(context.Background())
	if err != nil {
		fmt.Println("Error watch workers:", err)
		return EtcdMsg{}, err
	}
	// fmt.Println("watch debug", res, c.read)
	var Value string
	switch res.Action {
	case Expire:
		Value = res.PrevNode.Value
	case Set:
		fallthrough
	case Delete:
		Value = res.Node.Value
	default:
		fmt.Println("unknow etcd message", res)
	}
	return EtcdMsg{Value, res.Action}, nil
}

func (c *EtcdClient) Get(k string, option *client.GetOptions) (*client.Response, error) {
	key := alarmsDirect + "/" + k
	return c.kapi.Get(context.Background(), key, option)
}

func (c *EtcdClient) RecursiveGet(k string) (*client.Response, error) {
	if !strings.HasPrefix(k, "/"+alarmsDirect) {
		k = alarmsDirect + "/" + k
	}

	return c.kapi.Get(context.Background(), k, &client.GetOptions{Recursive: true})
}

func (c *EtcdClient) Set(k, v string, option *client.SetOptions) error {
	key := alarmsDirect + "/" + k
	_, err := c.kapi.Set(context.Background(), key, v, option)
	return err
}

func (c *EtcdClient) SetWithTTL(k, v string, duration time.Duration) error {
	key := alarmsDirect + "/" + k
	if duration == 0 {
		duration = 10 * time.Minute
	}
	_, err := c.kapi.Set(context.Background(), key, v, &client.SetOptions{TTL: duration})
	return err
}

func (c *EtcdClient) Delete(k string) error {
	key := k // NOTE: not add prefix
	_, err := c.kapi.Delete(context.Background(), key, nil)
	return err
}

func (c *EtcdClient) DeleteDir(k string) error {
	key := k // NOTE: not add prefix
	_, err := c.kapi.Delete(context.Background(), key, &client.DeleteOptions{Dir: true, Recursive: true})
	return err
}

func (c *EtcdClient) Listen() EtcdMsg {
	return <-c.MsgChannle
}

// TODO: set time limit. It will leak mem if not.
func (c *EtcdClient) Lock(k string, lockTime time.Duration) error {
	key := k + "/_lock"
	for {
		if err := c.Set(key, ".lock", &client.SetOptions{
			PrevExist: client.PrevNoExist,
			TTL:       lockTime}); err == nil {
			break
		}
		if _, err := c.Watch(key); err != nil {
			log.Error("etcd lock wait error: %s", err.Error())
			continue
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func (c *EtcdClient) Unlock(path string) error {
	lockKey := path + "/_lock"
	return c.Delete(lockKey)
}

func (c *EtcdClient) CreateDir(k string) error {
	return c.Set(k, k, &client.SetOptions{Dir: true})
}
