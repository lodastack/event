package loda

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/requests"

	"github.com/lodastack/log"
	m "github.com/lodastack/models"
)

const AlarmUri = "/api/v1/event/resource?ns=%s&type=alarm"

var PurgeChan chan string

type RespNS struct {
	Status int      `json:"httpstatus"`
	Data   []string `json:"data"`
}

type ResAlarm struct {
	HttpStatus int       `json:"httpstatus"`
	Data       []m.Alarm `json:"data"`
}

func init() {
	PurgeChan = make(chan string)
}

func PurgeAll() {
	var ticker *time.Ticker
	interval := config.GetConfig().Reg.ExpireDur
	if interval < 60 {
		interval = 60
	}
	duration := time.Duration(interval) * time.Second
	ticker = time.NewTicker(duration)
	for {
		select {
		case <-ticker.C:
		}
	}
}

func AllNS() ([]string, error) {
	var resNS RespNS
	var res []string
	url := fmt.Sprintf("%s/api/v1/event/ns?ns=&format=list", config.GetConfig().Reg.Link)

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return res, err
	}

	if resp.Status == 200 {
		err = json.Unmarshal(resp.Body, &resNS)
		if err != nil {
			log.Errorf("get all ns error: %s", err.Error())
			return res, err
		}
		return resNS.Data, nil
	}
	return res, fmt.Errorf("http status code: %d", resp.Status)
}

func getAlarmsMap(list []m.Alarm) map[string]m.Alarm {
	alarmMap := make(map[string]m.Alarm, len(list))
	for _, alarm := range list {
		alarmMap[alarm.Version] = alarm
	}
	return alarmMap
}

func GetAlarmsByNs(ns string) (map[string]m.Alarm, error) {
	var resAlarms ResAlarm
	resAlarms = ResAlarm{} // TODO

	url := fmt.Sprintf("%s"+AlarmUri, config.GetConfig().Reg.Link, ns)
	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get alarm of ns %s error: %s", ns, err.Error())
		return nil, err
	}

	if resp.Status != 200 {
		log.Errorf("get alarm of ns %s error: %+v", ns, resp)
		return nil, fmt.Errorf("query registry error")
	}
	err = json.Unmarshal(resp.Body, &resAlarms)
	return getAlarmsMap(resAlarms.Data), err
}
