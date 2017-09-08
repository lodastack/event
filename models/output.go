package models

import "time"

type AlertMsg struct {
	Receivers []string

	Ns          string
	Host        string
	IP          string
	Measurement string
	Level       string
	Tags        map[string]string
	Value       float64
	Time        time.Time
	AlarmName   string
	Expression  string

	Msg string
}

func NewAlertMsg(ns, host, ip, measurement, level string, alarmName string,
	expression string, receivers []string, tags map[string]string, value float64, time time.Time) AlertMsg {
	return AlertMsg{
		Ns:          ns,
		Host:        host,
		IP:          ip,
		Measurement: measurement,
		AlarmName:   alarmName,
		Expression:  expression,
		Level:       level,
		Receivers:   receivers,
		Tags:        tags,
		Value:       value,
		Time:        time,
	}
}
