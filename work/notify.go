package work

import (
	"errors"

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
		"1":      "CRIT",
		"2":      "WARN",
		"3":      "INFO",
	}
}

func send(alarmName, alarmLevel, expression, alertLevel, ip string, alertTypes []string, recievers []string, eventData models.EventData) error {
	if len(recievers) == 0 {
		return errors.New("empty recieve: ns:" + eventData.Ns + " Name:" + alarmName)
	}

	host, measurement := (*eventData.Data.Series[0]).Tags["host"], (*eventData.Data.Series[0]).Name
	tags := (*eventData.Data.Series[0]).Tags
	value := (*eventData.Data.Series[0]).Values[0][1].(float64)
	levelMsg, ok := levelMap[alertLevel]
	if !ok {
		levelMsg = levelMap["unknow"]
	}

	if err := sdkLog.Event(alarmName, eventData.Ns, measurement, alarmLevel, host,
		levelMsg, recievers, value); err != nil {
		log.Errorf("log alarm fail, error: %s, ns: %s, alert: %s", err.Error(), eventData.Ns, alarmName)
	}

	alertMsg := models.NewAlertMsg(
		eventData.Ns, host, ip, measurement,
		eventData.Level.String(), alarmName, expression, recievers, tags,
		value, eventData.Time)
	go sentToAlertHandler(alertLevel, alertTypes, alertMsg)
	return nil
}

// send the alertMsg to sms/mail/wechat handler.
func sentToAlertHandler(alertLevel string, alertType []string, noitfyData models.NotifyData) error {
	if alertLevel == "1" {
		alertType = append(alertType, "wechat")
	}
	alertType = common.RemoveDuplicateAndEmpty(alertType)

	for _, handler := range alertType {
		handlerFunc, ok := o.Handlers[handler]
		if !ok {
			log.Errorf("Unknow alert type %s.", handler)
			continue
		}
		if err := handlerFunc(noitfyData); err != nil {
			log.Errorf("output %s fail: %s", handler, err.Error())
			return err
		}
	}
	return nil
}
