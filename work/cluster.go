package work

import (
	"strings"
	"time"

	"github.com/coreos/etcd/client"
)

// Cluster is a simplified interface for manager data on etcd.
type Cluster interface {
	Get(k string, option *client.GetOptions) (*client.Response, error)
	Set(k, v string, option *client.SetOptions) error
	SetWithTTL(k, v string, duration time.Duration) error
	Remove(key string) error
	RemoveDir(k string) error
	RecursiveGet(k string) (*client.Response, error)
	Mkdir(k string) error
}

var (
	statusPath  = "status"
	hostPath    = "host"
	blockStatus = "status"
	blockTimes  = "blocktimes"

	etcdPrefix = "/loda-alarms"
)

// ReadEtcdLastSplit return the minimum dir/key of a etcd path.
func ReadEtcdLastSplit(etcdPath string) string {
	etcdKeySplit := strings.Split(etcdPath, "/")
	return etcdKeySplit[len(etcdKeySplit)-1]
}

// NsAbsPath add surfix to ns as the key of etcd.
func NsAbsPath(ns string) string {
	return etcdPrefix + "/" + ns
}

// AbsPath add surfix to path as the key of etcd.
func AbsPath(path string) string {
	return etcdPrefix + "/" + path
}

// StatusDir return the relative path by ns and alarm version.
// The path is uesd to manager machine stauts.
func StatusDir(ns, alarmVersion string) string {
	return ns + "/" + alarmVersion + "/" + statusPath
}

// HostStatusKey return the relative by ns,alarm version and host.
// The key is used as key to manage ns/alarm/host status.
func HostStatusKey(ns, alarmVersion, host string) string {
	return StatusDir(ns, alarmVersion) + "/" + host
}

// HostDir return the host relative path by ns and alarm version.
// The path is used to manage block status of machine.
func HostDir(ns, alarmVersion string) string {
	return ns + "/" + alarmVersion + "/" + hostPath
}

// HostKey return the host relative path by ns, alarm version and host.
// The dir is used to manage the block status and timse of one machine.
func HostKey(ns, alarmVersion, host string) string {
	return HostDir(ns, alarmVersion) + "/" + host
}

// BlockStatus return the alarm/host block status.
// The block status indicate the alarm/host is not/wait/already blocked.
func BlockStatusKey(ns, alarmVersion, host string) string {
	return HostKey(ns, alarmVersion, host) + "/" + blockStatus
}

// BlockTimes return the alarm/host block times.
// The block times indicates how many times the alarm/host is blocked,
// the values is used to deciede how long the this alarm/host will be blocked.
func BlockTimesKey(ns, alarmVersion, host string) string {
	return HostKey(ns, alarmVersion, host) + "/" + blockTimes
}
