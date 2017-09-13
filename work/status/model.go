package status

import (
	"sync"
	"time"

	"github.com/lodastack/event/models"
)

type (
	NS    string
	ALARM string
	HOST  string

	HostStatus  map[HOST]models.Status
	AlarmStatus map[ALARM]HostStatus
	NsStatus    map[NS]AlarmStatus
)

var statusData NsStatus = make(NsStatus)
var mu sync.RWMutex

func getNsStatusFromGlobal(ns NS) NsStatus {
	var output map[NS]AlarmStatus

	mu.RLock()
	if ns == "" {
		output = statusData
	} else {
		output = map[NS]AlarmStatus{}
		for _ns, alarmStatus := range statusData {
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

type (
	// WalkResult is result of walk status.
	WalkResult map[NS]interface{}

	// WalkFunc is the type of the function called for each HostStatus visited by Walk.
	WalkFunc func(ns NS, alarmVersion ALARM, host HOST, status string, result WalkResult)
)

// Walk walks the status, calling walkFunc for each HostStatus.
// If one ns/alarm has no alarm/host status, Walk pass zero param to walkFunc.
func (s *NsStatus) Walk(walkFunc WalkFunc) WalkResult {
	result := make(map[NS]interface{}, len(*s))
	for ns, alarmStatus := range *s {
		if len(alarmStatus) == 0 {
			walkFunc(ns, "", "", OK, result)
			continue
		}
		for alarmVersion, hostsStatus := range alarmStatus {
			if len(hostsStatus) == 0 {
				walkFunc(ns, alarmVersion, "", OK, result)
				continue
			}
			for host, hostStatus := range hostsStatus {
				walkFunc(ns, alarmVersion, host, hostStatus.Level, result)
			}
		}
	}
	return result
}

// GetNsStatus return map[string]bool reveal ns status.
// ns is identified by OK if has no alarmStatus.
func (s *NsStatus) getNsStatus() WalkResult {
	mu.RLock()
	defer mu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = true
		} else if status != OK {
			result[ns] = false
		}
	})
}

// getAlarmStatus return map[NS]map[ALARM]bool reveal alarm status.
// ns/alarm is identified by OK if has no hostStatus.
func (s *NsStatus) getAlarmStatus() WalkResult {
	mu.RLock()
	defer mu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = make(map[ALARM]bool)
		}

		if alarmVersion == "" {
			return
		}
		if _, exist := result[ns].(map[ALARM]bool)[alarmVersion]; !exist && status == OK {
			result[ns].(map[ALARM]bool)[alarmVersion] = true
		} else if status != OK {
			result[ns].(map[ALARM]bool)[alarmVersion] = false
		}
	})
}

// getNotOkHost return map[NS]map[HOST]bool reveal the not OK host.
func (s *NsStatus) getNotOkHost() WalkResult {
	mu.RLock()
	defer mu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = make(map[HOST]bool)
		}

		if host != "" && status != OK {
			result[ns].(map[HOST]bool)[host] = false
		}
	})
}

// GetStatusList return status list by level(OK, CRITICAL...).
func (s *NsStatus) getStatusList(level string) []models.Status {
	output := make([]models.Status, 0)
	mu.RLock()
	defer mu.RUnlock()
	for _, alarmStatus := range *s {
		for _, hostStatus := range alarmStatus {
			for _, hostStatus := range hostStatus {
				if hostStatus.Level == "" {
					continue
				}
				hostStatus.LastTime = ((time.Since(hostStatus.CreateTime)) / time.Second)
				if level == "" || hostStatus.Level == level {
					output = append(output, hostStatus)
				}
			}
		}
	}
	return output
}
