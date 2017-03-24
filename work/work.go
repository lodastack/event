package work

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lodastack/event/cluster"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
	m "github.com/lodastack/models"

	"github.com/lodastack/log"
)

var (
	interval     time.Duration = 20
	AllEventPath               = "all"
	timeFormat                 = "2006-01-02 15:04:05"

	nsPeroidDefault   = 5
	hostPeroidDefault = 5

	defaultNsBlock = 1
)

type Work struct {
	Cluster cluster.ClusterInf
}

func NewWork(c cluster.ClusterInf) *Work {
	return &Work{Cluster: c}
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

		// create alarm dir if not exit.
		for _, alarm := range alarms {
			alarmKey := ns + "/" + alarm.AlarmData.Version
			if _, err := w.Cluster.Get(alarmKey, nil); err != nil {
				log.Infof("get ns(%s) alarm(%s) fail: %s, set it and all dir.", ns, alarm.AlarmData.Version, err.Error())
				if err := w.Cluster.CreateDir(alarmKey); err != nil {
					log.Errorf("set ns(%s) alarm(%s) fail: %s, skip this alarm",
						ns, alarm.AlarmData.Version, err.Error())
					continue
				}
				allDirKey := alarmKey + "/" + AllEventPath
				if err := w.Cluster.CreateDir(allDirKey); err != nil {
					log.Errorf("set ns(%s) alarm(%s) dir \"all\" fail: %s, skip this alarm",
						ns, alarm.AlarmData.Version, err.Error())
					continue
				}
			}
			// check alarm watch
		}
	}
	return nil
}

func readEtcdLastSplit(etcdKey string) string {
	etcdKeySplit := strings.Split(etcdKey, "/")
	return etcdKeySplit[len(etcdKeySplit)-1]
}

func (w *Work) ReadAllNsBlock() {
	for {
		rep, err := w.Cluster.RecursiveGet("")
		if err != nil {
			log.Error("ReadAllNsBlock read root fail:", err.Error())
			continue
		}
		for _, nsNode := range rep.Node.Nodes {
			ns := readEtcdLastSplit(nsNode.Key)
			rep, err := w.Cluster.RecursiveGet(nsNode.Key)
			if err != nil {
				log.Errorf("ReadAllNsBlock read %s fail: %s", nsNode.Key, err.Error())
				continue
			}
			for _, alarmNode := range rep.Node.Nodes {
				alarmVersion := readEtcdLastSplit(alarmNode.Key)
				loda.Loda.RLock()
				alarm, ok := loda.Loda.NsAlarms[ns][alarmVersion]
				loda.Loda.RUnlock()
				if !ok {
					log.Errorf("Read ns %s alarm %s fail", ns, alarmVersion)
					continue
				}
				rep, err := w.Cluster.RecursiveGet(alarmNode.Key + "/" + AllEventPath)
				if err != nil {
					log.Error("ReadAllNsBlock read alarm fail:", err.Error())
					continue
				}
				if len(rep.Node.Nodes) != 0 {
					if len(rep.Node.Nodes) >= alarm.NsBlockTimes {
						msg := "报警收敛：</br>"
						for _, n := range rep.Node.Nodes {
							msg += readEtcdLastSplit(n.Key) + "</br>"
						}

						if err := sendMulit(
							strings.Split(alarm.AlarmData.Alert, ","),
							strings.Split(alarm.AlarmData.Groups, ","),
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
	go func() {
		for {
			alemVersion := <-loda.Loda.CleanChannel
			verSplit := m.SplitVersion(alemVersion)
			ns := verSplit[0] // TODO: check

			alarmPath := ns + "/" + alemVersion
			rep, err := w.Cluster.RecursiveGet(alarmPath)
			if err != nil {
				log.Error("Work CheckRegistryAlarmLoop get host fail:", err.Error())
				continue
			}
			for _, node := range rep.Node.Nodes {
				keySplit := strings.Split(node.Key, "/")
				if AllEventPath == keySplit[len(keySplit)-1] {
					continue
				}
				if err := w.Cluster.DeleteDir(node.Key); err != nil {
					log.Errorf("delete host block %s fail: %s", node.Key, err.Error())
				}
			}
		}
	}()

	//
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

	// ID format: "time:measurement:tags"
	block := false
	eventId := eventData.Time.Format(timeFormat) + ":" + eventData.ID
	allPath := ns + "/" + alarm.AlarmData.Version + "/" + AllEventPath
	if err := w.Cluster.SetWithTTL(
		allPath+"/"+eventId,
		eventData.Message,
		time.Duration(alarm.NsBlockPeriod)*time.Minute); err != nil {
		log.Errorf("set ns %s alarm %s fail: %s",
			ns, alarm.AlarmData.Version, err.Error())
	}
	if rep, err := w.Cluster.RecursiveGet(allPath); err != nil {
		log.Errorf("work HandleEvent get all path %s fail: %s", allPath, err.Error())
	} else {
		fmt.Printf("####  all: %+v %d\n", rep.Action, len(rep.Node.Nodes))
		if len(rep.Node.Nodes) >= alarm.NsBlockTimes {
			block = true
		}
	}

	hostPath := ns + "/" + alarm.AlarmData.Version + "/" + host
	if err := w.Cluster.SetWithTTL(
		hostPath+"/"+eventId,
		eventData.Message,
		time.Duration(alarm.HostBlockPeriod)*time.Minute); err != nil {
		log.Errorf("set ns %s alarm %s fail: %s",
			ns, alarm.AlarmData.Version, err.Error())
	}
	if rep, err := w.Cluster.RecursiveGet(hostPath); err != nil {
		log.Errorf("work HandleEvent get %s fail: %s", hostPath, err.Error())
	} else {
		fmt.Printf("####  host: %+v %d\n", rep.Action, len(rep.Node.Nodes))
		if !block && len(rep.Node.Nodes) <= alarm.HostBlockTimes {
			// read from alarm.AlarmData.Groups

			if err := sendOne(
				strings.Split(alarm.AlarmData.Alert, ","),
				strings.Split(alarm.AlarmData.Groups, ","),
				eventData); err != nil {
				log.Error("work output error:", err.Error())
			}
		}
	}

	return nil
}
