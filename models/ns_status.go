package models

import (
	"strings"
	"sync"
	"time"

	"github.com/lodastack/event/common"
)

type (
	NS    string
	ALARM string
	HOST  string

	HostStatus  map[HOST]Status
	AlarmStatus map[ALARM]HostStatus
	NsStatus    map[NS]AlarmStatus
)

var StatusData = make(NsStatus)
var StatusMu sync.RWMutex

func GetNsStatusFromGlobal(nsStr string) NsStatus {
	ns := NS(nsStr)
	var output map[NS]AlarmStatus

	StatusMu.RLock()
	if ns == "" {
		output = StatusData
	} else {
		output = map[NS]AlarmStatus{}
		for _ns, alarmStatus := range StatusData {
			if !strings.HasSuffix("."+string(_ns), "."+string(ns)) {
				continue
			}
			output[_ns] = alarmStatus
		}
	}
	StatusMu.RUnlock()
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
			walkFunc(ns, "", "", common.OK, result)
			continue
		}
		for alarmVersion, hostsStatus := range alarmStatus {
			if len(hostsStatus) == 0 {
				walkFunc(ns, alarmVersion, "", common.OK, result)
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
func (s *NsStatus) GetNsStatus() WalkResult {
	StatusMu.RLock()
	defer StatusMu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = true
		} else if status != common.OK {
			result[ns] = false
		}
	})
}

// getAlarmStatus return map[NS]map[ALARM]bool reveal alarm status.
// ns/alarm is identified by OK if has no hostStatus.
func (s *NsStatus) GetAlarmStatus() WalkResult {
	StatusMu.RLock()
	defer StatusMu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = make(map[ALARM]bool)
		}

		if alarmVersion == "" {
			return
		}
		if _, exist := result[ns].(map[ALARM]bool)[alarmVersion]; !exist && status == common.OK {
			result[ns].(map[ALARM]bool)[alarmVersion] = true
		} else if status != common.OK {
			result[ns].(map[ALARM]bool)[alarmVersion] = false
		}
	})
}

// getNotOkHost return map[NS]map[HOST]bool reveal the not OK host.
func (s *NsStatus) GetNotOkHost() WalkResult {
	StatusMu.RLock()
	defer StatusMu.RUnlock()
	return s.Walk(func(ns NS, alarmVersion ALARM, host HOST, status string, result WalkResult) {
		if _, existed := result[ns]; !existed {
			result[ns] = make(map[HOST]bool)
		}

		if host != "" && status != common.OK {
			result[ns].(map[HOST]bool)[host] = false
		}
	})
}

// GetStatusList return status list by level(OK, CRITICAL...).
func (s *NsStatus) GetStatusList(level string) []Status {
	output := make([]Status, 0)
	StatusMu.RLock()
	defer StatusMu.RUnlock()
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
