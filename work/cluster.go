package work

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/lodastack/event/config"
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

const (
	statusPath = "status"
	blockPath  = "block"

	blockStatus = "blockstatus"
	blockTimes  = "blocktimes"
	noneTags    = "none"
)

func isStatusPath(path string) bool {
	return ReadEtcdLastSplit(path) == statusPath
}

// ReadEtcdLastSplit return the minimum dir/key of a etcd path.
func ReadEtcdLastSplit(etcdPath string) string {
	etcdKeySplit := strings.Split(etcdPath, "/")
	return etcdKeySplit[len(etcdKeySplit)-1]
}

// NsAbsPath add surfix to ns as the key of etcd.
func NsAbsPath(ns string) string {
	return config.GetConfig().Etcd.Path + "/" + ns
}

// AbsPath add surfix to path as the key of etcd.
func AbsPath(path string) string {
	return config.GetConfig().Etcd.Path + "/" + path
}
func AlarmDir(ns, alarmVersion string) string {
	return ns + "/" + alarmVersion
}

func HostDir(ns, alarmVersion, host string) string {
	return AlarmDir(ns, alarmVersion) + "/" + host
}

func TagDir(ns, alarmVersion, host, tagString string) string {
	return HostDir(ns, alarmVersion, host) + "/" + tagString
}

// StatusKey return the relative path to keep alarm status of ns/alarm/tag.
func StatusKey(ns, alarmVersion, host, tagString string) string {
	return TagDir(ns, alarmVersion, host, tagString) + "/" + statusPath
}

// blockDir return the dir to keep block status and times of ns/alarm/tag.
func blockDir(ns, alarmVersion, host, tagString string) string {
	return TagDir(ns, alarmVersion, host, tagString) + "/" + blockPath
}

// BlockStatusKey return the ns/alarm/tag block status.
// The block status indicate the alarm/host is not/wait/already blocked.
func BlockStatusKey(ns, alarmVersion, host, tagString string) string {
	return blockDir(ns, alarmVersion, host, tagString) + "/" + blockStatus
}

// BlockTimesKey return the ns/alarm/tag block times.
// The block times indicates how many times the alarm/tag is blocked,
// the values is used to deciede how long the this alarm/host will be blocked.
func BlockTimesKey(ns, alarmVersion, host, tagString string) string {
	return blockDir(ns, alarmVersion, host, tagString) + "/" + blockTimes
}

func encodeTags(m map[string]string) string {
	// Empty maps marshal to empty bytes.
	if len(m) == 0 {
		return noneTags
	}

	// Extract keys and determine final size.
	sz := (len(m) * 2) - 1 // separators
	keys := make([]string, 0, len(m))
	for k, v := range m {
		keys = append(keys, k)
		sz += len(k) + len(v)
	}
	sort.Strings(keys)

	// Generate marshaled bytes.
	b := make([]byte, sz)
	buf := b
	for i, k := range keys {
		copy(buf, k)
		buf[len(k)] = '='
		buf = buf[len(k)+1:]

		v := m[k]
		copy(buf, v)
		if i < len(keys)-1 {
			buf[len(v)] = ';'
			buf = buf[len(v)+1:]
		}
	}

	return md5Byte2string(md5.Sum([]byte(b)))
}

func md5Byte2string(in [16]byte) string {
	tmp := make([]byte, 16)
	for _, value := range in {
		tmp = append(tmp, value)
	}

	return hex.EncodeToString(tmp[16:])
}
