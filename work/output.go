package work

import (
	"github.com/lodastack/event/common"
	"github.com/lodastack/event/models"
	o "github.com/lodastack/event/output"
	"github.com/lodastack/log"
)

var levelMap map[string]string

func init() {
	levelMap = map[string]string{
		"unknow": "Unknow Level",
		"OK":     "OK",
		"1":      "一级报警",
		"2":      "二级报警",
		"3":      "三级报警",
	}
}

func send(alarmName, expression, alertLevel, ip string, alertTypes []string, recievers []string, eventData models.EventData) error {
	host, measurement := (*eventData.Data.Series[0]).Tags["host"], (*eventData.Data.Series[0]).Name
	tags := (*eventData.Data.Series[0]).Tags
	value := (*eventData.Data.Series[0]).Values[0][1].(float64)
	levelMsg, ok := levelMap[alertLevel]
	if !ok {
		levelMsg = levelMap["unknow"]
	}

	if err := sdkLog.Event(alarmName, eventData.Ns, measurement, host,
		levelMsg, recievers, value); err != nil {
		log.Errorf("log alarm fail, error: %s, data: %+v", err.Error())
	}

	alertMsg := models.NewAlertMsg(
		eventData.Ns, host, ip, measurement,
		eventData.Level, alarmName, expression, recievers, tags,
		value, eventData.Time)
	go sentToAlertHandler(alertTypes, alertMsg)
	return nil
}

// send the alertMsg to sms/mail/wechat handler.
func sentToAlertHandler(alertType []string, alertMsg models.AlertMsg) error {
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
