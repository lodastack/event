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
	"github.com/lodastack/event/work/block"
	"github.com/lodastack/event/work/cluster"
	"github.com/lodastack/event/work/status"

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
	Cluster cluster.ClusterInf

	// query/set status.
	Status status.StatusInf

	// get/clear block status
	Block block.BlockInf
}

func NewWork(c cluster.ClusterInf) *Work {
	w := &Work{
		Cluster: c,
		Status:  status.NewStatus(c),
		Block:   block.NewBlock(c)}
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

func (w *Work) initAlarmDir(ns, alarmVersion string) error {
	alarmKey := ns + "/" + alarmVersion
	if err := w.Cluster.Mkdir(alarmKey); err != nil {
		log.Errorf("create alarm %s, %s dir fail", ns, alarmVersion)
	}

	statusDirKey := alarmKey + "/" + cluster.AlarmStatusPath
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
		loda.Loda.RLock()
		alarmNum := len(loda.Loda.NsAlarms)
		loda.Loda.RUnlock()
		if alarmNum != 0 {
			log.Info("loda resource init finished.")
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// read and check NsAlarm.
	for {
		time.Sleep(interval * time.Second)

		if err := w.CheckEtcdAlarms(); err != nil {
			log.Errorf("work loop error: %s", err)
		} else {
			log.Info("work loop success")
		}
	}
}

// UpdateAlarms init new alarm and remove the alarm in loda any more.
func (w *Work) CheckEtcdAlarms() error {
	loda.Loda.RLock()
	defer loda.Loda.RUnlock()
	// remove ns if not exist in loda.
	rep, err := w.Cluster.RecursiveGet("")
	if err != nil {
		log.Error("read root fail: %s", err.Error())
	} else {
		for _, nsNode := range rep.Node.Nodes {
			_ns := cluster.ReadEtcdLastSplit(nsNode.Key)
			if _, ok := loda.Loda.NsAlarms[_ns]; !ok {
				nsPath := cluster.NsAbsPath(_ns)
				log.Infof("cannot read ns %s on loda, remove it", _ns)
				if err := w.Cluster.RemoveDir(nsPath); err != nil {
					log.Errorf("delete ns %s fail: %s", nsPath, err.Error())
				}
			}
		}
	}

	// check the ns/alarm/host on etcd.
	for ns, alarms := range loda.Loda.NsAlarms {
		if rep, err := w.Cluster.RecursiveGet(ns); err != nil {
			// create ns dir if not exist.
			if len(alarms) == 0 {
				continue
			}
			log.Infof("get ns %s fail(%s) set it", ns, err.Error())
			if err := w.Cluster.Mkdir(ns); err != nil {
				log.Errorf("work set ns %s error: %s, skip this ns", ns, err.Error())
			}
		} else {
			// remove the alarm/host not existed in loda
			for _, alarmNode := range rep.Node.Nodes {
				alarmVersion := cluster.ReadEtcdLastSplit(alarmNode.Key)
				// remove the alarm not existed in loda
				if _, ok := loda.Loda.NsAlarms[ns][alarmVersion]; !ok {
					log.Infof("Read ns %s alarm %s fail, delete it", ns, alarmVersion)
					if err := w.Cluster.RemoveDir(alarmNode.Key); err != nil {
						log.Errorf("delete alarm path %s fail: %s", alarmNode.Key, err.Error())
					}
				} else {
					// remove the host not existed in loda
					alarmStatusKey := cluster.AlarmStatusDir(ns, alarmVersion)
					hostStatusNodes, err := w.Cluster.RecursiveGet(alarmStatusKey)
					if err != nil {
						log.Errorf("read etcd path fail %s", alarmStatusKey)
						continue
					}
					for _, hostNode := range hostStatusNodes.Node.Nodes {
						hostname := cluster.ReadEtcdLastSplit(hostNode.Key)
						if _, ok := loda.MachineIp(ns, hostname); !ok {
							log.Infof("cannot read ns %s hostname %s on loda, remove it", ns, hostname)
							statusPath := alarmNode.Key + "/" + cluster.AlarmStatusPath + "/" + hostname
							hostPath := alarmNode.Key + "/" + cluster.AlarmHostPath + "/" + hostname
							if err := w.Cluster.RemoveDir(statusPath); err != nil {
								log.Errorf("delete host %s fail: %s", statusPath, err.Error())
							}
							w.Cluster.RemoveDir(hostPath)
						}
					}
				}
			}
		}

		// create alarm if not exit.
		for _, alarm := range alarms {
			alarmKey := ns + "/" + alarm.AlarmData.Version
			if _, err := w.Cluster.Get(alarmKey, nil); err != nil {
				log.Infof("get ns(%s) alarm(%s) fail: %s, set it and all dir.", ns, alarm.AlarmData.Version, err.Error())
				if err := w.initAlarmDir(ns, alarm.AlarmData.Version); err != nil {
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

	// Log a new status if the status now exist.

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
	loda.Loda.RLock()
	alarm, ok := loda.Loda.NsAlarms[ns][alarmversion]
	loda.Loda.RUnlock()
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

	if loda.IsMachineOffline(ns, host) {
		log.Warningf("ns %s hostname %s is offline, not alert", ns, host)
		return nil
	}
	ip, _ := loda.MachineIp(ns, host)

	groups := strings.Split(alarm.AlarmData.Groups, ",")
	reveives := loda.GetGroupUsers(groups)
	if len(reveives) == 0 {
		return errors.New("empty recieve: " + strings.Join(groups, ","))
	}

	// update alarm status
	if err := w.setStatusAndLogToSDK(ns, alarm.AlarmData, host, ip, eventData.Level, reveives, eventData); err != nil {
		log.Errorf("set ns %s alarm %s host %s fail: %s",
			ns, alarm.AlarmData.Version, host, err.Error())
	}

	// read and check block/times
	if eventData.Level == status.OK {
		w.Block.ClearBlock(ns, alarm.AlarmData.Version, host)
		return send(
			alarm.AlarmData.Name,
			alarm.AlarmData.Level,
			alarm.AlarmData.Expression+alarm.AlarmData.Value,
			status.OK,
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
		log.Error("handler send event fail: %s", err.Error())
		return err
	}
	return nil
}
