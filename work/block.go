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
	noAction               int = -1
	noBlock                int = 0
	addBlock               int = 1
	alreadyAlertWhileBlock int = 2
)

type Block interface {
	// ClearBlock status/times by ns/alarmVersion/host.
	ClearBlock(ns, alarmVersion, hostname string, tag map[string]string) error

	// IsBlock check the ns/alarm/host is block or not, set the block status and times.
	IsBlock(ns string, alarm *loda.Alarm, hostname string, tag map[string]string) bool
}

type block struct {
	c Cluster
}

func NewBlock(c Cluster) Block {
	return &block{c: c}
}

// ClearBlock status/times by ns/alarmVersion/host.
func (b *block) ClearBlock(ns, alarmVersion, hostname string, tag map[string]string) error {
	alarmHostPath := AbsPath(TagDir(ns, alarmVersion, hostname, encodeTags(tag)))
	if err := b.c.RemoveDir(alarmHostPath); err != nil {
		if !strings.Contains(err.Error(), "Key not found") {
			return err
		}
		log.Errorf("del dir %s fail: %s", alarmHostPath, err.Error())
	}
	return nil
}

// IsBlock check the ns/alarm/host is block or not, set the block status and times.
func (b *block) IsBlock(ns string, alarm *loda.Alarm, hostname string, tag map[string]string) bool {
	tagString := encodeTags(tag)
	newBlockStatus, newBlockStatusTTL, newBlockTimes, newBlockTimesTTL, isBlock := b.readBlock(ns, alarm, hostname, tagString)
	if newBlockStatus != noAction && newBlockTimes != noAction {
		b.setBlockStatus(ns, alarm.AlarmData.Version, hostname, tagString, newBlockStatus, newBlockStatusTTL)
		b.setBlockTimes(ns, alarm.AlarmData.Version, hostname, tagString, newBlockTimes, newBlockTimesTTL)
	}
	return isBlock
}

// Read block status, return the next block status/times and if the ns/alarmVersion/host should be blocked or not.
// Block status, block times and their TTL will be treated in different way such as noBlock/addBlock/alreadyAlertWhileBlock.
// TimesTTL is statusTTL + alarm check interval, because event would happen not exactly which influence by net/machine and the other factors.
//
// noBlock:                The alarm happen first time, can not read any block status and times.
//
//	Do not block this alarm.
//	Set block times as 1 and set block status as alreadyAlertWhileBlock.
//
// addBlock:               Alarm occurs within one of the collect interval after last block, so can read block times and not read block status.
//
//	Do not block this alarm.
//	Only in this case can update block times as blocktimes+1, set status as alreadyAlertWhileBlock.
//	This status may be caused event happen at time which statusTTL timeout but in one alarm check interval.
//
// alreadyAlertWhileBlock: Alarm happen when block is not timeout,
//
//	Do nothing.
func (b *block) readBlock(ns string, alarm *loda.Alarm, hostname, tagString string) (
	newBlockStatus, newBlockStatusTTL, newBlockTimes, newTimesTTL int, isBlock bool) {
	var errStatus, errTimes error
	blockStatus, errStatus := b.getBlockStatus(ns, alarm.AlarmData.Version, hostname, tagString)
	if errStatus != nil {
		// Treat this alarm as first happen if block status not exist or read fail.
		blockStatus = noBlock
	}
	blockTimes, errTimes := b.getBlockTimes(ns, alarm.AlarmData.Version, hostname, tagString)

	// Do not block this event and set block times as block times+1 if the block has times but has no status.
	// Only in this case can the block times is updated to (last blocktimes + 1).
	if errStatus != nil && errTimes == nil && blockTimes > 0 {
		blockStatus = addBlock
	}

	// read alarm check interval, read as DefaultEvery if fail.
	e, err := strconv.Atoi(alarm.AlarmData.Every)
	if err != nil {
		e = DefaultInterval
	}

	switch blockStatus {
	case noBlock:
		newBlockStatusTTL, newTimesTTL = getBlockKeyTTL(alarm.BlockStep, 1, alarm.MaxStackTime, e)
		newBlockStatus, newBlockTimes, isBlock = alreadyAlertWhileBlock, 1, false

	case addBlock:
		newBlockStatus, newBlockTimes, isBlock = alreadyAlertWhileBlock, blockTimes+1, false
		newBlockStatusTTL, newTimesTTL = getBlockKeyTTL(alarm.BlockStep, blockTimes, alarm.MaxStackTime, e)

	case alreadyAlertWhileBlock:
		newBlockStatus, newBlockTimes = noAction, noAction
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

func (b *block) getBlockStatus(ns, alarmVersion, hostname, tagString string) (int, error) {
	return b.readFromCluster(BlockStatusKey(ns, alarmVersion, hostname, tagString))
}

func (b *block) getBlockTimes(ns, alarmVersion, hostname, tagString string) (int, error) {
	return b.readFromCluster(BlockTimesKey(ns, alarmVersion, hostname, tagString))
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
func (b *block) setBlockStatus(ns, alarmVersion, hostname, tagString string, status, statusTTL int) error {
	return b.c.SetWithTTL(
		BlockStatusKey(ns, alarmVersion, hostname, tagString),
		strconv.Itoa(int(status)),
		time.Duration(statusTTL)*time.Minute-5*time.Second)
}

// set block times with ttl for ns/alarmVersion/host.
func (b *block) setBlockTimes(ns, alarmVersion, hostname, tagString string, times, timesTTL int) error {
	return b.c.SetWithTTL(
		BlockTimesKey(ns, alarmVersion, hostname, tagString),
		strconv.Itoa(int(times)),
		time.Duration(timesTTL)*time.Minute+10*time.Second)
}
