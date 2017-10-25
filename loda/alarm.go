package loda

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/lodastack/event/common"
	"github.com/lodastack/event/config"
	"github.com/lodastack/event/requests"
	"github.com/lodastack/log"
	"github.com/lodastack/models"
)

// alarmURI is the URI to get resource.
const alarmURI = "/api/v1/event/resource?ns=%s&type=alarm"

var (
	defaultBlockStep = 10 // unit: minute
	maxBlockTime     = 60 // unit: minute

	// updateAlarmsInterval
	updateAlarmsInterval time.Duration = 2

	// Alarms represents all ns and its alarm resource.
	Alarms lodaAlarm
)

// UpdateAlarmsFromLoda read alarm resource from registry and update to global.
func UpdateAlarmsFromLoda() {
	for {
		if err := Alarms.updateAlarms(); err != nil {
			log.Errorf("loda ReadLoop fail: %s", err.Error())
		}
		time.Sleep(updateAlarmsInterval * time.Minute)
	}
}

func init() {
	Alarms = lodaAlarm{
		NsAlarms: make(map[string]map[string]*Alarm),
	}
}

// Alarm represent a alarm resource and its block property.
type Alarm struct {
	AlarmData    models.Alarm
	BlockStep    int
	MaxStackTime int
}

// newAlarm read the block property of the input alarm resource and return Alarm.
func newAlarm(alarm models.Alarm) *Alarm {
	output := Alarm{AlarmData: alarm}

	var err error
	if output.BlockStep, err = strconv.Atoi(alarm.BlockStep); err != nil || output.BlockStep < 1 {
		output.BlockStep = defaultBlockStep
	}
	if output.MaxStackTime, err = strconv.Atoi(alarm.MaxBlockTime); err != nil || output.MaxStackTime < 10 {
		output.MaxStackTime = maxBlockTime
	}
	return &output
}

type lodaAlarm struct {
	sync.RWMutex
	NsAlarms map[string]map[string]*Alarm
}

func (l *lodaAlarm) updateAlarms() error {
	allNs, err := allNS()
	if err != nil {
		fmt.Println("updateAlarms error:", err)
		return err
	}
	l.Lock()
	defer l.Unlock()

	// remove not exist ns from l
	for ns := range l.NsAlarms {
		if _, ok := common.ContainString(allNs, ns); !ok {
			delete(l.NsAlarms, ns)
		}
	}

	// update alarm of one ns
	for _, ns := range allNs {
		if _, ok := l.NsAlarms[ns]; !ok {
			l.NsAlarms[ns] = map[string]*Alarm{}
		}
		alarmMap, err := getAlarmsByNs(ns)
		if err != nil {
			log.Errorf("get alarm fail: %s", err.Error())
			return err
		}
		if len(alarmMap) == 0 {
			continue
		}

		// remove not exist alarm
		for oldAlarmVersion := range l.NsAlarms[ns] {
			if _, ok := alarmMap[oldAlarmVersion]; !ok {
				delete(l.NsAlarms[ns], oldAlarmVersion)
			}
		}

		// add new alarm
		for version, alarm := range alarmMap {
			_, ok := l.NsAlarms[ns][version]
			if !ok {
				l.NsAlarms[ns][alarm.Version] = newAlarm(alarm)
			}
		}

	}
	return nil
}

// RespAlarm is response from registry to get alarm resource.
type respAlarm struct {
	HTTPStatus int            `json:"httpstatus"`
	Data       []models.Alarm `json:"data"`
}

func getAlarmsMap(list []models.Alarm) map[string]models.Alarm {
	alarmMap := make(map[string]models.Alarm, len(list))
	for _, alarm := range list {
		alarmMap[alarm.Version] = alarm
	}
	return alarmMap
}

func getAlarmsByNs(ns string) (map[string]models.Alarm, error) {
	respAlarms := respAlarm{}

	url := fmt.Sprintf("%s"+alarmURI, config.GetConfig().Reg.Link, ns)
	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get alarm of ns %s error: %s", ns, err.Error())
		return nil, err
	}

	if resp.Status != 200 {
		log.Errorf("get alarm of ns %s error: %+v", ns, resp)
		return nil, fmt.Errorf("query registry error")
	}
	err = json.Unmarshal(resp.Body, &respAlarms)
	return getAlarmsMap(respAlarms.Data), err
}
