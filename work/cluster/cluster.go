package cluster

import (
	"strings"
	"time"

	"github.com/coreos/etcd/client"
)

type ClusterInf interface {
	Get(k string, option *client.GetOptions) (*client.Response, error)
	Set(k, v string, option *client.SetOptions) error
	SetWithTTL(k, v string, duration time.Duration) error
	Delete(key string) error
	DeleteDir(k string) error
	Lock(path string, lockTime time.Duration) error
	Unlock(path string) error
	RecursiveGet(k string) (*client.Response, error)
	CreateDir(k string) error
}

var (
	AlarmStatusPath = "status"
	AlarmHostPath   = "host"

	timeFormat = "2006-01-02 15:04:05"
	EtcdPrefix = "/loda-alarms" // TODO
)

func ReadEtcdLastSplit(etcdKey string) string {
	etcdKeySplit := strings.Split(etcdKey, "/")
	return etcdKeySplit[len(etcdKeySplit)-1]
}

func HostStatusKey(ns, alarmVersion, host string) string {
	return ns + "/" + alarmVersion + "/" + AlarmStatusPath + "/" + host
}

func AlarmStatusDir(ns, alarmVersion string) string {
	return ns + "/" + alarmVersion + "/" + AlarmStatusPath
}

func NsAbsPath(ns string) string {
	return EtcdPrefix + "/" + ns
}
