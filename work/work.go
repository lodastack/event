package work

import (
	"errors"
	"strings"
	"time"

	"github.com/lodastack/event/common"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"

	"github.com/lodastack/log"
	m "github.com/lodastack/models"
)

var (
	interval      time.Duration = 60
	timeFormat                  = "2006-01-02 15:04:05"
	alarmLevelMap               = map[string]string{"1": "CRIT", "2": "WARN"}
)

type Work struct {
	// Cluster used to get/set etcd.
	Cluster Cluster

	// query/set status from cluster.
	Status ClusterStatus

	// get/clear block status
	Block Block
}

func NewWork(c Cluster) *Work {
	w := &Work{
		Cluster: c,
		Status:  NewStatus(c),
		Block:   NewBlock(c)}

	go func() {
		for {
			if err := w.Status.GenGlobalStatus(); err != nil {
				log.Errorf("GenGlobalStatus: %s", err)
			}
			time.Sleep(10 * time.Second)
		}
	}()
	go w.CompareStatusAndLodaLoop()
	return w
}

// CompareStatusAndLodaLoop is the loop of compare status and loda,
// create new ns/alarm to etcd and remove the alarm not existed is loda.
func (w *Work) CompareStatusAndLodaLoop() {
	// wait loda init Loda.NsAlarm finished.
	for {
		loda.Alarms.RLock()
		alarmNum := len(loda.Alarms.NsAlarms)
		loda.Alarms.RUnlock()
		if alarmNum != 0 {
			log.Info("loda resource init finished.")
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// read and check NsAlarm.
	for {
		if err := w.compareStatusAndLoda(); err != nil {
			log.Errorf("work loop error: %s", err)
		} else {
			log.Info("work loop success")
		}

		time.Sleep(interval * time.Second)
	}
}

// compareStatusAndLoda create dir for new alarm and
// remove the alarm not exist in loda.
func (w *Work) compareStatusAndLoda() error {
	loda.Alarms.RLock()
	defer loda.Alarms.RUnlock()
	status := models.GetNsStatusFromGlobal("")
	for _ns := range status {
		nsInStatus := string(_ns)
		// remove ns not exist in loda.
		if _, ok := loda.Alarms.NsAlarms[nsInStatus]; !ok {
			log.Infof("cannot read ns %s on loda, remove it", nsInStatus)
			if err := w.Cluster.RemoveDir(NsAbsPath(nsInStatus)); err != nil {
				log.Errorf("remove ns %s fail: %s", nsInStatus, err.Error())
			}
			continue
		}

		for _alarmVersion := range status[_ns] {
			alarmVersionInStatus := string(_alarmVersion)
			// remove the alarm if not existed in loda
			if _, ok := loda.Alarms.NsAlarms[nsInStatus][alarmVersionInStatus]; !ok {
				log.Infof("Read ns %s alarm %s fail, delete it", nsInStatus, alarmVersionInStatus)
				alarmDirToRemove := AlarmDir(nsInStatus, alarmVersionInStatus)
				if err := w.Cluster.RemoveDir(AbsPath(alarmDirToRemove)); err != nil {
					log.Errorf("remove alarm path %s fail: %s", alarmDirToRemove, err.Error())
				}
				continue
			}

			// remove host if not exist in loda
			for _host := range status[_ns][_alarmVersion] {
				hostnameInStatus := string(_host)
				if _, ok := loda.MachineIP(nsInStatus, hostnameInStatus); ok {
					continue
				}
				hostDirToRemove := HostDir(nsInStatus, alarmVersionInStatus, hostnameInStatus)
				log.Infof("cannot read ns %s hostname %s on loda, path: %s, remove it", nsInStatus, hostnameInStatus, hostDirToRemove)
				if err := w.Cluster.RemoveDir(AbsPath(hostDirToRemove)); err != nil {
					log.Errorf("remove host path %s fail: %s", hostDirToRemove, err.Error())
				}
			}
		}
	}

	// create dir for new ns and
	// remove the ns/alarm/host from cluster if not exist in loda
	for nsInLoda, alarms := range loda.Alarms.NsAlarms {
		// create ns if not exist in etcd.
		if _, ok := status[models.NS(nsInLoda)]; !ok {
			log.Infof("get ns %s fail, set it", nsInLoda)
			if err := w.Cluster.Mkdir(nsInLoda); err != nil {
				log.Errorf("mkdir ns %s error: %s, skip this ns", nsInLoda, err.Error())
			}
		}

		// create alarm if not exist in etcd.
		for _, alarm := range alarms {
			alarmVersionInLoda := alarm.AlarmData.Version
			if _, ok := status[models.NS(nsInLoda)][models.ALARM(alarmVersionInLoda)]; !ok {
				log.Infof("get ns(%s) alarm(%s) fail, set it and all dir.", nsInLoda, alarmVersionInLoda)
				alarmKey := AlarmDir(nsInLoda, alarmVersionInLoda)
				if err := w.Cluster.Mkdir(alarmKey); err != nil {
					log.Errorf("create ns %s,alarm %s dir %s fail: %s", nsInLoda, alarmVersionInLoda, alarmKey, err.Error())
				}
			}
		}
	}
	return nil
}

// Set the status and log the status changes via sdkLog.
func (w *Work) setStatusAndLogToSDK(ns string, alarm m.Alarm, hostname, ip, level string, receives []string, eventData models.EventData) error {
	now := time.Now().Local()
	alarmLevel, _ := alarmLevelMap[alarm.Level]
	newStatus := models.Status{
		UpdateTime:   now,
		CreateTime:   now,
		Alarm:        alarmLevel,
		AlarmVersion: alarm.Version,
		Name:         alarm.Name,
		Measurement:  alarm.Measurement,
		Host:         hostname,
		Ip:           ip,
		Ns:           ns,
		Level:        level,

		Value:    common.SetPrecision((*eventData.Data.Series[0]).Values[0][1].(float64), 2),
		Tags:     (*eventData.Data.Series[0]).Tags,
		Reciever: loda.GetUserInfo(receives),
	}

	// Set the createtime of status by previous if the status is the same as previous.
	// Otherwise log the status change via sdkLog.
	if oldStatus, err := w.Status.GetStatusFromCluster(ns, alarm.Version, hostname, encodeTags(eventData.Tag())); err != nil {
		if err := sdkLog.NewStatus(alarm.Name, ns, alarm.Measurement, alarm.Level, hostname, level, receives, newStatus.Value); err != nil {
			log.Errorf("log status fail: %s", err.Error())
		}
	} else {
		if oldStatus.Level == newStatus.Level {
			newStatus.CreateTime = oldStatus.CreateTime
		} else {
			if err := sdkLog.StatusChange(alarm.Name, ns, alarm.Measurement, alarm.Level, hostname, oldStatus.Level, receives, newStatus.Value, oldStatus.CreateTime); err != nil {
				log.Errorf("log status fail: %s", err.Error())
			}
		}
	}
	return w.Status.SetStatus(ns, alarm, hostname, encodeTags(eventData.Tag()), newStatus)
}

func (w *Work) HandleEvent(ns, alarmversion string, eventData models.EventData) error {
	loda.Alarms.RLock()
	alarm, ok := loda.Alarms.NsAlarms[ns][alarmversion]
	loda.Alarms.RUnlock()
	if !ok {
		log.Errorf("read ns %s alarm %s alarm data error", ns, alarmversion)
		return errors.New("event process error: not have alarm data")
	}
	eventData.Time = eventData.Time.Local()
	eventData.Ns = ns

	host, ok := eventData.Host()
	if !ok {
		log.Debug("event data has no host, maybe cluster alarm.")
	}

	if loda.IsOfflineMachine(ns, host) {
		log.Warningf("ns %s hostname %s is offline, not alert", ns, host)
		return nil
	}
	ip, _ := loda.MachineIP(ns, host)

	groups := strings.Split(alarm.AlarmData.Groups, ",")
	reveives := loda.GetGroupUsers(groups)
	if len(reveives) == 0 {
		return errors.New("empty recieve: " + strings.Join(groups, ","))
	}

	// update alarm status
	if err := w.setStatusAndLogToSDK(ns, alarm.AlarmData, host, ip, eventData.Level.String(), reveives, eventData); err != nil {
		log.Errorf("set ns %s alarm %s host %s fail: %s",
			ns, alarm.AlarmData.Version, host, err.Error())
	}

	// read and check block/times
	if eventData.Level.String() == common.OK {
		w.Block.ClearBlock(ns, alarm.AlarmData.Version, host, eventData.Tag())
		return send(
			alarm.AlarmData.Name,
			alarm.AlarmData.Level,
			alarm.AlarmData.Expression+alarm.AlarmData.Value,
			common.OK,
			ip,
			strings.Split(alarm.AlarmData.Alert, ","),
			reveives,
			eventData)
	}

	if w.Block.IsBlock(ns, alarm, host, eventData.Tag()) {
		return nil
	}

	if err := send(
		alarm.AlarmData.Name,
		alarm.AlarmData.Level,
		alarm.AlarmData.Expression+alarm.AlarmData.Value,
		alarm.AlarmData.Level,
		ip,
		strings.Split(alarm.AlarmData.Alert, ","),
		reveives,
		eventData); err != nil {
		log.Errorf("handler send event fail: %s", err.Error())
		return err
	}
	return nil
}
