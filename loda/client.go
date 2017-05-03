package loda

import (
	"fmt"
	"sync"
	"time"

	"github.com/lodastack/event/common"

	"github.com/lodastack/log"
)

var (
	readRegInterval time.Duration = 1
	Loda            lodaAlarm
)

type lodaAlarm struct {
	sync.RWMutex
	NsAlarms map[string]map[string]*Alarm

	CleanChannel chan string
}

func init() {
	Loda = lodaAlarm{
		NsAlarms:     make(map[string]map[string]*Alarm),
		CleanChannel: make(chan string, 0),
	}
}

func (l *lodaAlarm) UpdateAlarms() error {
	allNs, err := AllNS()
	if err != nil {
		fmt.Println("UpdateAlarms error:", err)
		return err
	}
	l.Lock()
	defer l.Unlock()

	for ns := range l.NsAlarms {
		// check removed ns
		if _, ok := common.ContainString(allNs, ns); !ok {
			for version := range l.NsAlarms[ns] {
				l.NsAlarms[ns][version].Stop()
			}
			delete(l.NsAlarms, ns)
		}
	}

	for _, ns := range allNs {
		if _, ok := l.NsAlarms[ns]; !ok {
			l.NsAlarms[ns] = map[string]*Alarm{}
		}
		alarmMap, err := GetAlarmsByNs(ns)
		if err != nil {
			log.Errorf("get alarm fail: %s", err.Error())
			return err
		}
		if len(alarmMap) == 0 {
			continue
		}
		// check new alarm
		for version, alarm := range alarmMap {
			_, ok := l.NsAlarms[ns][version]
			if !ok {
				l.NsAlarms[ns][alarm.Version] = newAlarm(alarm)
				go l.NsAlarms[ns][alarm.Version].Run(l.CleanChannel)
			}
		}

		// check removed alarm
		for oldAlarmVersion := range l.NsAlarms[ns] {
			if _, ok := alarmMap[oldAlarmVersion]; !ok {
				l.NsAlarms[ns][oldAlarmVersion].Stop()
				delete(l.NsAlarms[ns], oldAlarmVersion)
			}
		}

	}
	return nil
}

func ReadLoop() error {
	for {
		if err := Loda.UpdateAlarms(); err != nil {
			log.Errorf("loda ReadLoop fail: %s", err.Error())
		}
		time.Sleep(readRegInterval * time.Minute)
	}
	return nil
}
