package work

import (
	"errors"
	// "strconv"
	"strings"
	"time"

	"github.com/lodastack/event/cluster"
	"github.com/lodastack/event/common"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
	"github.com/lodastack/log"
	m "github.com/lodastack/models"

	"github.com/coreos/etcd/client"
)

var (
	interval        time.Duration = 60
	AlarmStatusPath               = "status"
	AlarmHostPath                 = "host"

	timeFormat        = "2006-01-02 15:04:05"
	etcdPrefix        = "/loda-alarms" // TODO
	nsPeroidDefault   = 5
	hostPeroidDefault = 5

	defaultNsBlock = 1
	alarmLevelMap  = map[string]string{"1": "一级报警", "2": "二级报警"}
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

type Work struct {
	Cluster ClusterInf
}

func NewWork(c cluster.ClusterInf) *Work {
	w := &Work{Cluster: c}
	w.makeStatus()
	go func() {
		c := time.Tick(10 * time.Second)
		for {
			select {
			case <-c:
				w.makeStatus()
			}
		}
	}()
	return w
}

func (w *Work) createDir(dir string) error {
	err := w.Cluster.CreateDir(dir)
	if err != nil {
		log.Errorf("create dir %s fail: %s", dir, err.Error())
	}
	return err
}

func (w *Work) initAlarmDir(ns, alarmVersion string) error {
	alarmKey := ns + "/" + alarmVersion
	if err := w.createDir(alarmKey); err != nil {
		log.Errorf("create alarm %s, %s dir fail", ns, alarmVersion)
	}

	statusDirKey := alarmKey + "/" + AlarmStatusPath
	if err := w.createDir(statusDirKey); err != nil {
		log.Errorf("create alarm %s, %s status dir fail", ns, statusDirKey)
	}
	return nil
}

func readEtcdLastSplit(etcdKey string) string {
	etcdKeySplit := strings.Split(etcdKey, "/")
	return etcdKeySplit[len(etcdKeySplit)-1]
}

func readHostFromEtcdKey(etcdKey string) string {
	etcdKeySplit := strings.Split(etcdKey, ":")
	return etcdKeySplit[len(etcdKeySplit)-1]
}

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

	// loda update NsAlarm loop.
	for {
		time.Sleep(interval * time.Second)

		if err := w.CheckEtcdAlarms(); err != nil {
			log.Errorf("work loop error: %s", err)
		} else {
			log.Info("work loop success")
		}
	}
}

// UpdateAlarms init new alarm and delete removed alarm in etcd.
func (w *Work) CheckEtcdAlarms() error {
	loda.Loda.RLock()
	defer loda.Loda.RUnlock()

	// delete ns if not exist in loda.
	rep, err := w.Cluster.RecursiveGet("")
	if err != nil {
		log.Error("read root fail: %s", err.Error())
	} else {
		for _, nsNode := range rep.Node.Nodes {
			_ns := readEtcdLastSplit(nsNode.Key)
			if _, ok := loda.Loda.NsAlarms[_ns]; !ok {
				nsPath := etcdPrefix + "/" + _ns
				log.Infof("cannot read ns %s on loda, remove it", _ns)
				if err := w.Cluster.DeleteDir(nsPath); err != nil {
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
			if err := w.Cluster.CreateDir(ns); err != nil {
				log.Errorf("work set ns %s error: %s, skip this ns", ns, err.Error())
			}
		} else {
			for _, alarmNode := range rep.Node.Nodes {
				alarmVersion := readEtcdLastSplit(alarmNode.Key)
				if _, ok := loda.Loda.NsAlarms[ns][alarmVersion]; !ok {
					// delete the alarm not in loda
					log.Infof("Read ns %s alarm %s fail, delete it", ns, alarmVersion)
					if err := w.Cluster.DeleteDir(alarmNode.Key); err != nil {
						log.Errorf("delete alarm path %s fail: %s", alarmNode.Key, err.Error())
					}
				} else {
					// delete the host not in loda
					alarmKey := ns + "/" + alarmVersion + "/" + AlarmStatusPath
					hostStatusNodes, err := w.Cluster.RecursiveGet(alarmKey)
					if err != nil {
						log.Errorf("read etcd path fail %s", alarmKey)
						continue
					}
					for _, hostNode := range hostStatusNodes.Node.Nodes {
						hostname := readEtcdLastSplit(hostNode.Key)
						if loda.MachineIp(ns, hostname) != "" {
							continue
						}
						log.Infof("cannot read ns %s hostname %s on loda, remove it", ns, hostname)
						statusPath := alarmNode.Key + "/" + AlarmStatusPath + "/" + hostname
						if err := w.Cluster.DeleteDir(statusPath); err != nil {
							log.Errorf("delete host %s fail: %s", statusPath, err.Error())
						}
						hostPath := alarmNode.Key + "/" + AlarmHostPath + "/" + hostname
						w.Cluster.DeleteDir(hostPath)
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

func (w *Work) setAlarmStatus(ns string, alarm m.Alarm, host, ip, level string, receives []string, eventData models.EventData) error {
	now := time.Now().Local()
	alarmLevel, _ := alarmLevelMap[alarm.Level]
	newStatus := models.Status{
		UpdateTime:  now,
		CreateTime:  now,
		Alarm:       alarmLevel,
		Name:        alarm.Name,
		Measurement: alarm.Measurement,
		Host:        host,
		Ip:          ip,
		Ns:          ns,
		Level:       level,

		Value:    common.SetPrecision((*eventData.Data.Series[0]).Values[0][1].(float64), 2),
		Tags:     (*eventData.Data.Series[0]).Tags,
		Reciever: receives,
	}

	statusPath := ns + "/" + alarm.Version + "/" + AlarmStatusPath + "/" + host
	if rep, err := w.Cluster.RecursiveGet(statusPath); err == nil {
		if oldStatus, err := models.NewStatusByString(rep.Node.Value); err == nil {
			if oldStatus.Level == newStatus.Level {
				newStatus.CreateTime = oldStatus.CreateTime
			} else {
				if err := logOneStatus(newStatus); err != nil {
					log.Errorf("log status fail: %s", err.Error())
				}
			}
		}
	} else {
		if err := logNewStatus(newStatus); err != nil {
			log.Errorf("log status fail: %s", err.Error())
		}
	}
	statusString, _ := newStatus.String()

	return w.Cluster.Set(
		statusPath,
		statusString,
		&client.SetOptions{})
}

// handleEventToHostPath handle event to alarm-host path, and check if the alert block by alert host level.
func (w *Work) handleEvent(ns, version, host, ip, eventID string, eventData models.EventData, alarm *loda.Alarm, reveives []string) error {
	if err := sendOne(
		alarm.AlarmData.Name,
		alarm.AlarmData.Expression+alarm.AlarmData.Value,
		alarm.AlarmData.Level,
		ip,
		strings.Split(alarm.AlarmData.Alert, ","),
		reveives,
		eventData); err != nil {
		log.Error("work output error:", err.Error())
		return err
	}
	return nil
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
	ip := loda.MachineIp(ns, host)

	groups := strings.Split(alarm.AlarmData.Groups, ",")
	reveives := GetRevieves(groups)
	if len(reveives) == 0 {
		return errors.New("empty recieve: " + strings.Join(groups, ","))
	}

	// update alarm status
	if err := w.setAlarmStatus(ns, alarm.AlarmData, host, ip, eventData.Level, reveives, eventData); err != nil {
		log.Errorf("set ns %s alarm %s host %s fail: %s",
			ns, alarm.AlarmData.Version, host, err.Error())
	}

	// read and check block/times
	if eventData.Level == OK {
		w.clearBlock(ns, alarm.AlarmData.Version, host)
		return sendOne(
			alarm.AlarmData.Name,
			alarm.AlarmData.Expression+alarm.AlarmData.Value,
			OK,
			ip,
			strings.Split(alarm.AlarmData.Alert, ","),
			reveives,
			eventData)
	}

	if isBlock := w.readBlockStatus(ns, alarm, host); isBlock {
		return nil
	}

	// ID format: "time:measurement:tags"
	// handle event by ns-all
	eventId := eventData.Time.Format(timeFormat) + ":" + eventData.ID + ":" + host
	if err := w.handleEvent(ns, alarm.AlarmData.Version, host, ip, eventId, eventData, alarm, reveives); err != nil {
		log.Errorf("handle event by host path fail: %s", err.Error())
		return err
	}
	return nil
}
