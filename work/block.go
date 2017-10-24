package work

import (
	"strconv"
	"strings"
	"time"

	"github.com/lodastack/event/loda"
	"github.com/lodastack/log"

	"github.com/coreos/etcd/client"
)

const (
	// DefaultInterval is alarm check interval. unit: minute
	DefaultInterval int = 1

	// block status
	noBlock                int = 0
	addBlock               int = 1
	alreadyAlertWhileBlock int = 2
)

type Block interface {
	// ClearBlock status/times by ns/alarmVersion/host.
	ClearBlock(ns, version, host string) error

	// IsBlock check the ns/alarm/host is block or not, set the block status and times.
	IsBlock(ns string, alarm *loda.Alarm, host string) bool
}

type block struct {
	c Cluster
}

func NewBlock(c Cluster) Block {
	return &block{c: c}
}

// ClearBlock status/times by ns/alarmVersion/host.
func (b *block) ClearBlock(ns, alarmVersion, host string) error {
	alarmHostPath := AbsPath(HostKey(ns, alarmVersion, host))
	// alarmHostPath := cluster.EtcdPrefix + "/" + ns + "/" + version + "/" + cluster.AlarmHostPath + "/" + host
	if err := b.c.RemoveDir(alarmHostPath); err != nil {
		if !strings.Contains(err.Error(), "Key not found") {
			return err
		}
		log.Errorf("del dir %s fail: %s", alarmHostPath, err.Error())
	}
	return nil
}

// IsBlock check the ns/alarm/host is block or not, set the block status and times.
func (b *block) IsBlock(ns string, alarm *loda.Alarm, host string) bool {
	status, statusTTL, blockTimes, timesTTL, isBlock := b.readBlock(ns, alarm, host)
	if status != 0 {
		b.setBlockStatus(ns, alarm.AlarmData.Version, host, status, statusTTL)
	}
	if blockTimes != 0 {
		b.setBlockTimes(ns, alarm.AlarmData.Version, host, blockTimes, timesTTL)
	}
	return isBlock
}

// Read block status, return the next block status/times and if the ns/alarmVersion/host should be blocked or not.
// Block status, block times and their TTL will be treated in different way such as noBlock/addBlock/alreadyAlertWhileBlock.
// TimesTTL is statusTTL + alarm check interval, because event would happen not exactly which influence by net/machine and the other factors.
//
// noBlock:                has no block status and times, not block this event,
//                         set times as 1 and set status as alreadyAlertWhileBlock.
// addBlock:               has no status but has times, not block this event,
//                         set times as times+1 and set status as alreadyAlertWhileBlock.
//                         this status may be caused event happen at time which statusTTL timeout but in one alarm check interval.
// alreadyAlertWhileBlock: event happen when block is not timeout,
//                         block this event and set times +1 which cause the TTL + (alarm check interval).
func (b *block) readBlock(ns string, alarm *loda.Alarm, host string) (status, statusTTL, blockTimes, timesTTL int, isBlock bool) {
	var errStatus, errTimes error
	status, errStatus = b.getBlockStatus(ns, alarm.AlarmData.Version, host)
	blockTimes, errTimes = b.getBlockTimes(ns, alarm.AlarmData.Version, host)

	// do not block this event and set block times as block times+1 if the block has times but has no status.
	if errStatus != nil && errTimes == nil && blockTimes > 0 {
		status = addBlock
	}

	// read alarm check interval, read as DefaultEvery if fail.
	e, err := strconv.Atoi(alarm.AlarmData.Every)
	if err != nil {
		e = DefaultInterval
	}

	switch status {
	case noBlock:
		statusTTL, timesTTL = getBlockKeyTTL(alarm.BlockStep, 1, alarm.MaxStackTime, e)
		status, blockTimes, isBlock = alreadyAlertWhileBlock, 1, false

	case addBlock:
		status, blockTimes, isBlock = alreadyAlertWhileBlock, blockTimes+1, false
		statusTTL, timesTTL = getBlockKeyTTL(alarm.BlockStep, blockTimes, alarm.MaxStackTime, e)

	case alreadyAlertWhileBlock:
		status, blockTimes = 0, 0
		isBlock = true
	}

	return
}

// return TTL of block status and block times.
// statusTTL is (block times) * step
// timesTTL is statusTTL+ (alarm check interval)
func getBlockKeyTTL(step, times, max, alarmCheckInterval int) (statusTTL, timesTTL int) {
	statusTTL = step * times
	if statusTTL == 0 {
		statusTTL = 5
	}

	if statusTTL > max {
		statusTTL = max
	}
	timesTTL = statusTTL + alarmCheckInterval
	return
}

func (b *block) getBlockStatus(ns, alarmVersion, host string) (int, error) {
	return b.readFromCluster(BlockStatusKey(ns, alarmVersion, host))
}

func (b *block) getBlockTimes(ns, alarmVersion, host string) (int, error) {
	return b.readFromCluster(BlockTimesKey(ns, alarmVersion, host))
}

func (b *block) readFromCluster(k string) (int, error) {
	resp, err := b.c.Get(k, &client.GetOptions{})
	if err != nil {
		return 0, err
	}

	v, err := strconv.Atoi(resp.Node.Value)
	if err != nil {
		log.Debugf("conv value %s fail, value: %s, error: %s, read as 0", k, resp.Node.Value, err.Error())
		v = 0
	}
	return v, nil
}

// set block status with ttl for ns/alarmVersion/host.
func (b *block) setBlockStatus(ns, alarmVersion, host string, status, statusTTL int) error {
	return b.c.SetWithTTL(
		BlockStatusKey(ns, alarmVersion, host),
		strconv.Itoa(int(status)),
		time.Duration(statusTTL)*time.Minute-5*time.Second)
}

// set block times with ttl for ns/alarmVersion/host.
func (b *block) setBlockTimes(ns, alarmVersion, host string, times, timesTTL int) error {
	return b.c.SetWithTTL(
		BlockTimesKey(ns, alarmVersion, host),
		strconv.Itoa(int(times)),
		time.Duration(timesTTL)*time.Minute+10*time.Second)
}
