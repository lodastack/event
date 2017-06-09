package work

import (
	"errors"
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
	interval        time.Duration = 20
	AllEventPath                  = "all"
	AlarmStatusPath               = "status"
	AlarmHostPath                 = "host"

	timeFormat        = "2006-01-02 15:04:05"
	etcdPrefix        = "/loda-alarms" // TODO
	nsPeroidDefault   = 5
	hostPeroidDefault = 5

	defaultNsBlock = 1
	alarmLevelMap  = map[string]string{"1": "一级报警", "2": "二级报警"}
)

type Work struct {
	Cluster cluster.ClusterInf
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
	allDirKey := alarmKey + "/" + AllEventPath
	if err := w.createDir(statusDirKey); err != nil {
		log.Errorf("create alarm %s, %s status dir fail", ns, statusDirKey)
	}
	if err := w.createDir(allDirKey); err != nil {
		log.Errorf("create alarm %s, %s all dir fail", ns, allDirKey)
	}
	return nil
}

func (w *Work) UpdateAlarms() error {
	loda.Loda.RLock()
	defer loda.Loda.RUnlock()
	for ns, alarms := range loda.Loda.NsAlarms {
		if len(alarms) == 0 {
			continue
		}
		// create ns dir if not exist.
		if _, err := w.Cluster.Get(ns, nil); err != nil {
			log.Infof("get ns %s fail(%s) set it", ns, err.Error())
			if err := w.Cluster.CreateDir(ns); err != nil {
				log.Errorf("work set ns %s error: %s, skip this ns", ns, err.Error())
				continue
			}
		}

		// create alarm and alarm/all dir if not exit.
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

func readEtcdLastSplit(etcdKey string) string {
	etcdKeySplit := strings.Split(etcdKey, "/")
	return etcdKeySplit[len(etcdKeySplit)-1]
}

func readHostFromEtcdKey(etcdKey string) string {
	etcdKeySplit := strings.Split(etcdKey, ":")
	return etcdKeySplit[len(etcdKeySplit)-1]
}

func (w *Work) ReadAllNsBlock() {
	for {
		rep, err := w.Cluster.RecursiveGet("")
		if err != nil {
			log.Error("ReadAllNsBlock read root fail:", err.Error())
			continue
		}
		// loop etcd ns path
		for _, nsNode := range rep.Node.Nodes {
			ns := readEtcdLastSplit(nsNode.Key)
			rep, err := w.Cluster.RecursiveGet(nsNode.Key)
			if err != nil {
				log.Errorf("ReadAllNsBlock read %s fail: %s", nsNode.Key, err.Error())
				continue
			}
			// loop alarm of the ns
			for _, alarmNode := range rep.Node.Nodes {
				alarmVersion := readEtcdLastSplit(alarmNode.Key)
				loda.Loda.RLock()
				alarm, ok := loda.Loda.NsAlarms[ns][alarmVersion]
				loda.Loda.RUnlock()
				if !ok {
					log.Errorf("Read ns %s alarm %s fail, delete it", ns, alarmVersion)
					if err := w.Cluster.DeleteDir(alarmNode.Key); err != nil {
						log.Errorf("delete alarm path %s fail: %s", alarmNode.Key, err.Error())
					}
					continue
				}
				rep, err := w.Cluster.RecursiveGet(alarmNode.Key + "/" + AllEventPath)
				if err != nil {
					continue
				}
				if len(rep.Node.Nodes) != 0 {
					if len(rep.Node.Nodes) >= alarm.NsBlockTimes {
						hosts := make([]string, len(rep.Node.Nodes))
						for index, n := range rep.Node.Nodes {
							hosts[index] = readHostFromEtcdKey(n.Key)
						}
						msg := "Host:  " +
							strings.Join(common.RemoveDuplicateAndEmpty(hosts), ",\n")

						groups := strings.Split(alarm.AlarmData.Groups, ",")
						reveives := GetRevieves(groups)
						if len(reveives) == 0 {
							log.Errorf("empty recieve: " + strings.Join(groups, ","))
							continue
						}
						if err := sendMulit(
							ns,
							alarm.AlarmData.Name,
							strings.Split(alarm.AlarmData.Alert, ","),
							reveives,
							msg); err != nil {
							log.Error("work output error:", err.Error())
						}
						if err := w.Cluster.DeleteDir(alarmNode.Key + "/" + AllEventPath); err != nil {
							log.Errorf("delete all block %s fail: %s", alarmNode.Key+"/"+AllEventPath, err.Error())
						}
					}
				}
			}
		}
		time.Sleep(time.Duration(defaultNsBlock) * time.Minute) // TODO
	}
}

func (w *Work) CheckRegistryAlarmLoop() {
	// clean host path every host-block-Peroid, otherwise new alert will block by old alert.
	go func() {
		for {
			alemVersion := <-loda.Loda.CleanChannel
			verSplit := m.SplitVersion(alemVersion)
			ns := verSplit[0] // TODO: check

			alarmHostPath := etcdPrefix + "/" + ns + "/" + alemVersion + "/" + AlarmHostPath
			if err := w.Cluster.DeleteDir(alarmHostPath); err != nil {
				if !strings.Contains(err.Error(), "Key not found") {
					log.Errorf("Work CheckRegistryAlarmLoop delete host %s fail: %s", alarmHostPath, err.Error())
				}
			}
		}
	}()

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
	go w.ReadAllNsBlock()
	for {
		if err := w.UpdateAlarms(); err != nil {
			log.Errorf("work loop error: %s", err)
		} else {
			log.Info("work loop success")
		}

		time.Sleep(interval * time.Second)
	}
}

func (w *Work) setAlarmStatus(ns string, alarm m.Alarm, host, level string, receives []string, eventData models.EventData) error {
	now := time.Now().Local()
	alarmLevel, _ := alarmLevelMap[alarm.Level]
	newStatus := models.Status{
		UpdateTime:  now,
		CreateTime:  now,
		Alarm:       alarmLevel,
		Name:        alarm.Name,
		Measurement: alarm.Measurement,
		Host:        host,
		Ns:          ns,
		Level:       level,

		Value:    common.SetPrecision((*eventData.Data.Series[0]).Values[0][1].(float64), 2),
		Tags:     (*eventData.Data.Series[0]).Tags,
		Ip:       loda.MachineIp(host),
		Reciever: receives,
	}

	statusPath := ns + "/" + alarm.Version + "/" + AlarmStatusPath + "/" + host
	if rep, err := w.Cluster.RecursiveGet(statusPath); err == nil {
		if oldStatus, err := models.NewStatusByString(rep.Node.Value); err == nil {
			if oldStatus.Level == newStatus.Level {
				newStatus.CreateTime = oldStatus.CreateTime
			}
		}
	}
	newStatus.LastTime = newStatus.UpdateTime.Sub(newStatus.CreateTime).Minutes()
	statusString, _ := newStatus.String()

	return w.Cluster.Set(
		statusPath,
		statusString,
		&client.SetOptions{})
}

// handEventToAllPath handle event to alarm-ns path, and check if the alert block by alert ns level.
func (w *Work) handleEventToAllPath(ns, version, eventID, message string, blockPeriod, blockTimes int) (error, bool) {
	allPath := ns + "/" + version + "/" + AllEventPath
	if err := w.Cluster.SetWithTTL(
		allPath+"/"+eventID,
		message,
		time.Duration(blockPeriod)*time.Minute); err != nil {
		log.Errorf("set ns %s alarm %s to all path fail: %s",
			ns, version, err.Error())
	}

	// check if the alert block by ns level.
	rep, err := w.Cluster.RecursiveGet(allPath)
	if err != nil {
		log.Errorf("work HandleEvent get all path %s fail: %s", allPath, err.Error())
		return err, false
	}
	if len(rep.Node.Nodes) >= blockTimes {
		return nil, true
	}
	return nil, false
}

// handleEventToHostPath handle event to alarm-host path, and check if the alert block by alert host level.
func (w *Work) handleEventToHostPath(ns, version, host, eventID string, eventData models.EventData, alarm *loda.Alarm, blockByNS bool, reveives []string) error {
	hostPath := ns + "/" + version + "/" + AlarmHostPath + "/" + host
	if err := w.Cluster.SetWithTTL(
		hostPath+"/"+eventID,
		eventData.Message,
		time.Duration(alarm.HostBlockPeriod)*time.Minute); err != nil {
		log.Errorf("set ns %s alarm %s to host path fail: %s",
			ns, version, err.Error())
	}

	rep, err := w.Cluster.RecursiveGet(hostPath)
	if err != nil {
		log.Errorf("work HandleEvent to host path get %s fail: %s", hostPath, err.Error())
		return err
	}

	if !blockByNS && len(rep.Node.Nodes) <= alarm.HostBlockTimes {
		if err := sendOne(
			alarm.AlarmData.Name,
			// TODO: relative/deadman
			alarm.AlarmData.Expression+alarm.AlarmData.Value,
			alarm.AlarmData.Level,
			strings.Split(alarm.AlarmData.Alert, ","),
			reveives,
			eventData); err != nil {
			log.Error("work output error:", err.Error())
			return err
		}
	}
	return nil
}

func (w *Work) HandleEvent(ns, alarmversion string, eventData models.EventData) error {
	alarm, ok := loda.Loda.NsAlarms[ns][alarmversion]
	if !ok {
		log.Errorf("read ns %s alarm %s alarm data error", ns, alarmversion)
		return errors.New("event process error: not have alarm data")
	}
	eventData.Time = eventData.Time.Local()
	eventData.Ns = ns

	host, ok := eventData.Host()
	if !ok {
		log.Errorf("event data has no host: %+v", eventData)
		return errors.New("event has no host")
	}

	if loda.IsMachineOffline(ns, host) {
		log.Warningf("ns %s hostname %s is offline, not alert", ns, host)
		return nil
	}

	groups := strings.Split(alarm.AlarmData.Groups, ",")
	reveives := GetRevieves(groups)
	if len(reveives) == 0 {
		return errors.New("empty recieve: " + strings.Join(groups, ","))
	}

	// update alarm status
	if err := w.setAlarmStatus(ns, alarm.AlarmData, host, eventData.Level, reveives, eventData); err != nil {
		log.Errorf("set ns %s alarm %s host %s fail: %s",
			ns, alarm.AlarmData.Version, host, err.Error())
	}

	if eventData.Level == OK {
		return sendOne(
			alarm.AlarmData.Name,
			alarm.AlarmData.Expression+alarm.AlarmData.Value,
			OK,
			strings.Split(alarm.AlarmData.Alert, ","),
			strings.Split(alarm.AlarmData.Groups, ","),
			eventData)
	}

	// ID format: "time:measurement:tags"
	// handle event by ns-all
	eventId := eventData.Time.Format(timeFormat) + ":" + eventData.ID + ":" + host
	err, block := w.handleEventToAllPath(ns, alarm.AlarmData.Version, eventId, eventData.Message, alarm.NsBlockPeriod, alarm.NsBlockTimes)
	if err != nil {
		log.Errorf("handle event by all path fail: %s", err.Error())
	}

	// handle event by ns-host
	if err := w.handleEventToHostPath(ns, alarm.AlarmData.Version, host, eventId, eventData, alarm, block, reveives); err != nil {
		log.Errorf("handle event by host path fail: %s", err.Error())
		return err
	}
	return nil
}
