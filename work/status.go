package work

import (
	"sync"
	"time"

	"github.com/lodastack/event/models"
	"github.com/lodastack/log"
)

type (
	NS    string
	ALARM string
	HOST  string

	HostStatus  map[HOST]models.Status
	AlarmStatus map[ALARM]HostStatus
	NsStatus    map[NS]AlarmStatus
)

const (
	OK = "OK"
)

var StatusData NsStatus = make(NsStatus)
var mu sync.RWMutex

func (s *NsStatus) copy(ns NS) NsStatus {
	var output map[NS]AlarmStatus

	mu.RLock()
	if ns == "" {
		output = make(map[NS]AlarmStatus, len(StatusData))
		for _ns, AlarmStatus := range StatusData {
			output[_ns] = AlarmStatus
		}
	} else {
		output = map[NS]AlarmStatus{}
		for _ns, alarmStatus := range StatusData {
			if len(_ns) < len(ns) || _ns[len(_ns)-len(ns):] != ns ||
				(len(_ns) > len(ns) && _ns[len(_ns)-len(ns)-1] != '.') {
				continue
			}
			output[_ns] = alarmStatus
		}
	}
	mu.RUnlock()
	return output
}

func (s *NsStatus) Detail(level string) []models.Status {
	output := make([]models.Status, 0)
	for _, alarmStatus := range *s {
		for _, hostStatus := range alarmStatus {
			for _, hostStatus := range hostStatus {
				if hostStatus.Level == "" {
					continue
				}
				hostStatus.LastTime = (((time.Since(hostStatus.CreateTime)) / time.Second) * time.Second).String()
				if level == "" || hostStatus.Level == level {
					output = append(output, hostStatus)
				}
			}
		}
	}
	return output
}

func (s *NsStatus) CheckByAlarm(ns string) map[NS]map[ALARM]bool {
	output := make(map[NS]map[ALARM]bool)
	for _ns, alarmStatus := range *s {
		output[_ns] = make(map[ALARM]bool, len(alarmStatus))
		for alarmVersion, hostStatus := range alarmStatus {
			for _, hostStatus := range hostStatus {
				if hostStatus.Level != OK {
					output[_ns][alarmVersion] = false
					goto next
				}
			}
			output[_ns][alarmVersion] = true
		next:
			continue
		}
	}
	return output
}

func (s *NsStatus) CheckByHost(ns string) map[NS]map[HOST]bool {
	output := make(map[NS]map[HOST]bool)
	for _ns, alarmStatus := range *s {
		output[_ns] = make(map[HOST]bool, len(alarmStatus))
		for _, hostsStatus := range alarmStatus {
			for host, hostStatus := range hostsStatus {
				if hostStatus.Level != OK {
					output[_ns][host] = false
				}
			}
		}
	}
	return output
}

func (s *NsStatus) CheckByNs() map[NS]bool {
	output := make(map[NS]bool, len(StatusData))
	for ns, alarmStatus := range StatusData {
		for _, hostsStatus := range alarmStatus {
			for _, hostStatus := range hostsStatus {
				if hostStatus.Level != OK {
					output[ns] = false
					goto next
				}
			}
			output[ns] = true
		}
	next:
		continue
	}
	return output
}

func (w *Work) HandleStatus(ns string) (NsStatus, error) {
	return StatusData.copy(NS(ns)), nil
}

func (w *Work) makeStatus() error {
	data := make(NsStatus)
	if err := w.getNsPathList(&data); err != nil {
		log.Error("HandleStatus get ns fail: %s", err.Error())
		return err
	}

	if err := w.getAlarmList(&data); err != nil {
		log.Error("HandleStatus get alarm fail: %s", err.Error())
		return err
	}

	if err := w.getHostStatus(&data); err != nil {
		log.Error("HandleStatus get alarm fail: %s", err.Error())
		return err
	}
	mu.Lock()
	StatusData = data
	mu.Unlock()
	return nil
}

func (w *Work) getNsPathList(status *NsStatus) error {
	rep, err := w.Cluster.RecursiveGet("")
	if err != nil {
		log.Errorf("work HandleStatus get root fail: %s", err.Error())
		return err
	}

	for _, node := range rep.Node.Nodes {
		(*status)[NS(readEtcdLastSplit(node.Key))] = make(map[ALARM]HostStatus)
	}
	return nil
}

func (w *Work) getAlarmList(status *NsStatus) error {
	for ns := range *status {
		rep, err := w.Cluster.RecursiveGet(string(ns))
		if err != nil {
			log.Errorf("work HandleStatus get ns %s fail: %s", ns, err.Error())
			continue
			// return err ?
		}
		for _, node := range rep.Node.Nodes {
			alarmVersion := readEtcdLastSplit(node.Key)
			(*status)[ns][ALARM(alarmVersion)] = HostStatus{}
		}
	}
	return nil
}

func (w *Work) getHostStatus(status *NsStatus) error {
	for ns := range *status {
		for alarmVersion := range (*status)[ns] {
			rep, err := w.Cluster.RecursiveGet(string(ns) + "/" + string(alarmVersion) + "/" + AlarmStatusPath)
			if err != nil {
				log.Errorf("work HandleStatus get ns %s alarm %s fail: %s", ns, alarmVersion, err.Error())
				continue
				// return err ?
			}
			for _, node := range rep.Node.Nodes {
				host := readEtcdLastSplit(node.Key)
				if node.Value != "" {
					hostStatus, err := models.NewStatusByString(node.Value)
					if err != nil {
						log.Errorf("unmarshal ns %s alarm %s status fail: %s", ns, alarmVersion, err.Error())
					}
					(*status)[ns][alarmVersion][HOST(host)] = hostStatus
				}
			}
		}
	}
	return nil
}
