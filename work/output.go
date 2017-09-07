package work

import (
	"github.com/lodastack/event/common"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
	o "github.com/lodastack/event/output"
	"github.com/lodastack/log"
)

var levelMap map[string]string

func init() {
	levelMap = map[string]string{
		"OK": "OK",
		"1":  "一级报警",
		"2":  "二级报警",
		"3":  "三级报警",
	}
}

func GetRevieves(groups []string) []string {
	recieves := make([]string, 0)
	for _, gname := range groups {
		users, err := loda.GetUserByGroup(gname)
		if err != nil {
			continue
		}
		recieves = append(recieves, users...)
	}
	recieves = common.RemoveDuplicateAndEmpty(recieves)
	if len(recieves) == 0 {
		return nil
	}
	return recieves
}

func sendOne(alarmName, expression, alertLevel, ip string, alertTypes []string, recieves []string, eventData models.EventData) error {
	return send(alertTypes, recieves, alertLevel, models.NewAlertMsg(
		eventData.Ns,
		(*eventData.Data.Series[0]).Tags["host"],
		ip,
		(*eventData.Data.Series[0]).Name,
		eventData.Level,
		alarmName,
		expression,
		(*eventData.Data.Series[0]).Tags,
		(*eventData.Data.Series[0]).Values[0][1].(float64),
		eventData.Time),
	)
}

func send(alertTypes, recieves []string, alertLevel string, alertMsg models.AlertMsg) error {
	alertMsg.Users = recieves

	if levelMsg, ok := levelMap[alertLevel]; ok {
		go func(name, ns, measurement, host, level string, users []string, value float64) {
			if err := sdkLog.Event(name, ns, measurement, host, level, users, value); err != nil {
				log.Errorf("log alarm fail, error: %s, data: %+v", err.Error(), alertMsg)
			}
		}(alertMsg.AlarmName, alertMsg.Ns, alertMsg.Measurement, alertMsg.Host, levelMsg, alertMsg.Users, alertMsg.Value)
	}

	go output(alertTypes, alertMsg)
	return nil
}

func output(alertType []string, alertMsg models.AlertMsg) error {
	alertType = common.RemoveDuplicateAndEmpty(alertType)

	for _, handler := range alertType {
		handlerFunc, ok := o.Handlers[handler]
		if !ok {
			log.Error("Unknow alert type %s.", handler)
			continue
		}
		if err := handlerFunc(alertMsg); err != nil {
			log.Error("output %s fail: %s", handler, err.Error())
			return err
		}
	}
	return nil
}
