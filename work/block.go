package work

import (
	"strconv"
	"strings"
	"time"

	"github.com/lodastack/event/loda"
	"github.com/lodastack/log"

	"github.com/coreos/etcd/client"
	"github.com/lodastack/event/work/cluster"
)

const (
	block      = "status"
	blockTimes = "blocktimes"

	DefaultEvery = 1 // unit: minute
)

type blockStatus int

var (
	noBlock                blockStatus = 0
	addBlock               blockStatus = 1
	alreadyAlertWhileBlock blockStatus = 2
)

func getBlockKeyTTL(step, times, max, every int) (statusTTL, timesTTL int) {
	statusTTL = step * times
	if statusTTL == 0 {
		statusTTL = 5
	}

	if statusTTL > max {
		statusTTL = max
	}
	timesTTL = statusTTL + every
	return
}

func (w *Work) getBlockStatus(ns, version, host string) (blockStatus, error) {
	hostPath := ns + "/" + version + "/" + cluster.AlarmHostPath + "/" + host
	statusPath := hostPath + "/" + block
	resp, err := w.Cluster.Get(statusPath, &client.GetOptions{})
	if err != nil {
		return blockStatus(0), err
	}

	status, err := strconv.Atoi(resp.Node.Value)
	if err != nil {
		log.Debugf("conv status %s fail: %s, read as 0", resp.Node.Value, err.Error())
		status = 0
	}
	return blockStatus(status), nil
}

func (w *Work) getBlockTimes(ns, version, host string) (int, error) {
	hostPath := ns + "/" + version + "/" + cluster.AlarmHostPath + "/" + host
	statusPath := hostPath + "/" + blockTimes
	resp, err := w.Cluster.Get(statusPath, &client.GetOptions{})
	if err != nil {
		return 0, err
	}

	times, err := strconv.Atoi(resp.Node.Value)
	if err != nil {
		log.Debugf("conv status %s fail: %s, read as 0", resp.Node.Value, err.Error())
		times = 0
	}
	return times, nil
}

func (w *Work) setBlockStatus(ns, version, host string, status blockStatus, statusTTL int) error {
	hostPath := ns + "/" + version + "/" + cluster.AlarmHostPath + "/" + host

	return w.Cluster.SetWithTTL(
		hostPath+"/"+block,
		strconv.Itoa(int(status)),
		time.Duration(statusTTL)*time.Minute-5*time.Second)
}

func (w *Work) updateBlockTimes(ns, version, host string, times, timesTTL int) error {
	hostPath := ns + "/" + version + "/" + cluster.AlarmHostPath + "/" + host

	return w.Cluster.SetWithTTL(
		hostPath+"/"+blockTimes,
		strconv.Itoa(int(times)),
		time.Duration(timesTTL)*time.Minute+10*time.Second)
}

func (w *Work) clearBlock(ns, version, host string) error {
	alarmHostPath := cluster.EtcdPrefix + "/" + ns + "/" + version + "/" + cluster.AlarmHostPath + "/" + host
	if err := w.Cluster.DeleteDir(alarmHostPath); err != nil {
		if !strings.Contains(err.Error(), "Key not found") {
			return err
		} else {
			log.Errorf("del dir %s fail: %s", alarmHostPath, err.Error())
		}
	}

	return nil
}

// read block status, return the next block status/times.
func (w *Work) readBlock(ns string, alarm *loda.Alarm, host string) (status blockStatus, statusTTL, blockTimes, timesTTL int, isBlock bool) {
	var errStatus, errTimes error
	status, errStatus = w.getBlockStatus(ns, alarm.AlarmData.Version, host)
	blockTimes, errTimes = w.getBlockTimes(ns, alarm.AlarmData.Version, host)
	if errStatus != nil && errTimes == nil && blockTimes > 0 {
		status = addBlock
	}

	every, err := strconv.Atoi(alarm.AlarmData.Every)
	if err != nil {
		every = DefaultEvery
	}

	switch status {
	case noBlock:
		statusTTL, timesTTL = getBlockKeyTTL(alarm.BlockStep, 1, alarm.MaxStackTime, every)
		status, blockTimes, isBlock = alreadyAlertWhileBlock, 1, false

	case addBlock:
		status, blockTimes, isBlock = alreadyAlertWhileBlock, blockTimes+1, false
		statusTTL, timesTTL = getBlockKeyTTL(alarm.BlockStep, blockTimes, alarm.MaxStackTime, every)

	case alreadyAlertWhileBlock:
		status, blockTimes = 0, 0
		isBlock = true
	}

	return
}

// check the alsert is block or not, set block stauts and times.
func (w *Work) readBlockStatus(ns string, alarm *loda.Alarm, host string) bool {
	status, statusTTL, blockTimes, timesTTL, isBlock := w.readBlock(ns, alarm, host)
	if status != 0 {
		w.setBlockStatus(ns, alarm.AlarmData.Version, host, status, statusTTL)
	}
	if blockTimes != 0 {
		w.updateBlockTimes(ns, alarm.AlarmData.Version, host, blockTimes, timesTTL)
	}
	return isBlock
}
