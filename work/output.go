package work

import (
	"errors"

	"github.com/lodastack/event/common"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
	o "github.com/lodastack/event/output"
	"github.com/lodastack/log"
)

func sendOne(alarmName, expression string, alertTypes []string, groups []string, eventData models.EventData) error {
	// TODO: error
	send(alertTypes, groups, models.NewAlertMsg(
		eventData.Ns,
		(*eventData.Data.Series[0]).Tags["host"],
		(*eventData.Data.Series[0]).Name,
		eventData.Level,
		alarmName,
		expression,
		(*eventData.Data.Series[0]).Tags,
		(*eventData.Data.Series[0]).Values[0][1].(float64),
		eventData.Time),
	)
	return nil
}

func sendMulit(ns, alarmName string, alertTypes []string, groups []string, msg string) error {
	alertMsg := models.AlertMsg{AlarmName: alarmName, Ns: ns, Msg: msg}
	send(alertTypes, groups, alertMsg)
	return nil
}

func send(alertTypes []string, groups []string, alertMsg models.AlertMsg) error {
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
		return errors.New("empty recieve")
	}
	alertMsg.Users = recieves
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
			return err
		}
	}
	return nil
}
