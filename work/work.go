package work

import (
	"errors"
	// "strconv"
	"strings"
	"time"

	// "github.com/lodastack/event/cluster"
	"github.com/lodastack/event/common"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"

	// "github.com/coreos/etcd/client"
	"github.com/lodastack/log"
	m "github.com/lodastack/models"
)

var (
	interval      time.Duration = 60
	timeFormat                  = "2006-01-02 15:04:05"
	alarmLevelMap               = map[string]string{"1": "一级报警", "2": "二级报警"}
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
	w.Status.GenGlobalStatus()
	go func() {
		c := time.Tick(10 * time.Second)
		for {
			select {
			case <-c:
				w.Status.GenGlobalStatus()
			}
		}
	}()
	return w
}

// mkdirForAlarm create dir on cluster for alarm to manager host and block status.
func (w *Work) mkdirForAlarm(ns, alarmVersion string) error {
	alarmKey := ns + "/" + alarmVersion
	if err := w.Cluster.Mkdir(alarmKey); err != nil {
		log.Errorf("create alarm %s, %s dir fail", ns, alarmVersion)
	}

	statusDirKey := alarmKey + "/" + statusPath
	if err := w.Cluster.Mkdir(statusDirKey); err != nil {
		log.Errorf("create alarm %s, %s status dir fail", ns, statusDirKey)
	}
	return nil
}

// CheckAlarmLoop is the loop to read alarm from loda,
// create new alarm on cluster and remove the alarm not existed.
func (w *Work) CheckAlarmLoop() {
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
		time.Sleep(interval * time.Second)

		if err := w.CheckAlarmsOnEtcd(); err != nil {
			log.Errorf("work loop error: %s", err)
		} else {
			log.Info("work loop success")
		}
	}
}

// CheckAlarmsOnEtcd create dir for new alarm and
// remove the alarm not exist in loda.
func (w *Work) CheckAlarmsOnEtcd() error {
	loda.Alarms.RLock()
	defer loda.Alarms.RUnlock()

	// remove ns if not exist in loda.
	rep, err := w.Cluster.RecursiveGet("")
	if err != nil {
		log.Errorf("read alarm fail: %s", err.Error())
	} else {
		for _, nsNode := range rep.Node.Nodes {
			_ns := ReadEtcdLastSplit(nsNode.Key)
			if _, ok := loda.Alarms.NsAlarms[_ns]; ok {
				continue
			}
			log.Infof("cannot read ns %s on loda, remove it", _ns)
			if err := w.Cluster.RemoveDir(NsAbsPath(_ns)); err != nil {
				log.Errorf("delete ns %s fail: %s", _ns, err.Error())
			}
		}
	}

	// create dir for new ns and
	// remove the ns/alarm/host from cluster if not exist in loda
	for ns, alarms := range loda.Alarms.NsAlarms {
		rep, err := w.Cluster.RecursiveGet(ns)
		// create ns dir if get fail.
		if err != nil {
			if len(alarms) == 0 {
				continue
			}
			log.Infof("get ns %s fail(%s) set it", ns, err.Error())
			if err := w.Cluster.Mkdir(ns); err != nil {
				log.Errorf("work set ns %s error: %s, skip this ns", ns, err.Error())
			}
			continue
		}

		// check the alarm of this ns
		for _, alarmNode := range rep.Node.Nodes {
			alarmVersion := ReadEtcdLastSplit(alarmNode.Key)
			// remove the alarm if not existed in loda
			if _, ok := loda.Alarms.NsAlarms[ns][alarmVersion]; !ok {
				log.Infof("Read ns %s alarm %s fail, delete it", ns, alarmVersion)
				if err := w.Cluster.RemoveDir(alarmNode.Key); err != nil {
					log.Errorf("delete alarm path %s fail: %s", alarmNode.Key, err.Error())
				}
				continue
			}

			hostStatusNodes, err := w.Cluster.RecursiveGet(StatusDir(ns, alarmVersion))
			// skip alarm which not has status path. the alarm maybe a new alarm or not has alert.
			if err != nil {
				log.Errorf("read etcd path fail, ns: %s, alarm: %s", ns, alarmVersion)
				continue
			}
			// remove the host if not existed in loda
			for _, hostNode := range hostStatusNodes.Node.Nodes {
				hostname := ReadEtcdLastSplit(hostNode.Key)
				if _, ok := loda.MachineIP(ns, hostname); ok {
					continue
				}
				log.Infof("cannot read ns %s hostname %s on loda, remove it", ns, hostname)
				statusPath := alarmNode.Key + "/" + statusPath + "/" + hostname
				hostPath := alarmNode.Key + "/" + hostPath + "/" + hostname
				if err := w.Cluster.RemoveDir(statusPath); err != nil {
					log.Errorf("delete status %s fail: %s", statusPath, err.Error())
				}
				if err := w.Cluster.RemoveDir(hostPath); err != nil {
					log.Errorf("delete host %s fail: %s", statusPath, err.Error())
				}
			}
		}

		// create alarm if not exit.
		for _, alarm := range alarms {
			alarmKey := ns + "/" + alarm.AlarmData.Version
			if _, err := w.Cluster.Get(alarmKey, nil); err != nil {
				log.Infof("get ns(%s) alarm(%s) fail: %s, set it and all dir.", ns, alarm.AlarmData.Version, err.Error())
				if err := w.mkdirForAlarm(ns, alarm.AlarmData.Version); err != nil {
					log.Errorf("init ns %s alarm %s fail", ns, alarm.AlarmData.Version)
					continue
				}
			}
		}
	}
	return nil
}

// Set the status and log the status changes via sdkLog.
func (w *Work) setStatusAndLogToSDK(ns string, alarm m.Alarm, host, ip, level string, receives []string, eventData models.EventData) error {
	now := time.Now().Local()
	alarmLevel, _ := alarmLevelMap[alarm.Level]
	newStatus := models.Status{
		UpdateTime:   now,
		CreateTime:   now,
		Alarm:        alarmLevel,
		AlarmVersion: alarm.Version,
		Name:         alarm.Name,
		Measurement:  alarm.Measurement,
		Host:         host,
		Ip:           ip,
		Ns:           ns,
		Level:        level,

		Value:    common.SetPrecision((*eventData.Data.Series[0]).Values[0][1].(float64), 2),
		Tags:     (*eventData.Data.Series[0]).Tags,
		Reciever: loda.GetUserInfo(receives),
	}

	// Set the createtime of status by previous if the status is the same as previous.
	// Otherwise log the status change via sdkLog.
	if oldStatus, err := w.Status.GetStatusFromCluster(ns, alarm.Version, host); err != nil {
		if err := sdkLog.NewStatus(alarm.Name, ns, alarm.Measurement, alarm.Level, host, level, receives, newStatus.Value); err != nil {
			log.Errorf("log status fail: %s", err.Error())
		}
	} else {
		if oldStatus.Level == newStatus.Level {
			newStatus.CreateTime = oldStatus.CreateTime
		} else {
			if err := sdkLog.StatusChange(alarm.Name, ns, alarm.Measurement, alarm.Level, host, oldStatus.Level, receives, newStatus.Value, oldStatus.CreateTime); err != nil {
				log.Errorf("log status fail: %s", err.Error())
			}
		}
	}
	return w.Status.SetStatus(ns, alarm, host, newStatus)
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
		w.Block.ClearBlock(ns, alarm.AlarmData.Version, host)
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

	if w.Block.IsBlock(ns, alarm, host) {
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
