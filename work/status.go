package work

import (
	"sync"

	"github.com/lodastack/log"
)

type (
	NS    string
	ALARM string
	HOST  string

	HostStatus  map[HOST]string
	AlarmStatus map[ALARM]HostStatus
	NsStatus    map[NS]AlarmStatus
)

const (
	OK = "OK"
)

var Status NsStatus = make(NsStatus)
var mu sync.RWMutex

func (s *NsStatus) copy(ns NS) NsStatus {
	var output map[NS]AlarmStatus

	mu.RLock()
	if ns == "" {
		output = make(map[NS]AlarmStatus, len(Status))
		for _ns, AlarmStatus := range Status {
			output[_ns] = AlarmStatus
		}
	} else {
		output = map[NS]AlarmStatus{}
		for _ns, alarmStatus := range Status {
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

func (s *NsStatus) CheckByAlarm(ns string) map[NS]map[ALARM]bool {
	output := make(map[NS]map[ALARM]bool)
	for _ns, alarmStatus := range *s {
		output[_ns] = make(map[ALARM]bool, len(alarmStatus))
		for alarmVersion, hostStatus := range alarmStatus {
			for _, status := range hostStatus {
				if status != OK {
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
		for _, hostStatus := range alarmStatus {
			for host, status := range hostStatus {
				if status != OK {
					output[_ns][host] = false
				}
			}
		}
	}
	return output
}

func (s *NsStatus) CheckByNs() map[NS]bool {
	output := make(map[NS]bool, len(Status))
	for ns, alarmStatus := range Status {
		for _, hostStatus := range alarmStatus {
			for _, status := range hostStatus {
				if status != OK {
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
	return Status.copy(NS(ns)), nil
}

func (w *Work) makeStatus() error {
	status := make(NsStatus)
	if err := w.getNsPathList(&status); err != nil {
		log.Error("HandleStatus get ns fail: %s", err.Error())
		return err
	}

	if err := w.getAlarmList(&status); err != nil {
		log.Error("HandleStatus get alarm fail: %s", err.Error())
		return err
	}

	if err := w.getHostStatus(&status); err != nil {
		log.Error("HandleStatus get alarm fail: %s", err.Error())
		return err
	}
	mu.Lock()
	Status = status
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
				(*status)[ns][alarmVersion][HOST(host)] = node.Value
			}
		}
	}
	return nil
}
