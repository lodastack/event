package models

import "time"

type AlertMsg struct {
	Users []string

	Ns          string
	Host        string
	Measurement string
	Level       string
	Tags        map[string]string
	Value       float64
	Time        time.Time

	Msg string
}

func NewAlertMsg(ns, host, measurement, level string,
	tags map[string]string, value float64, time time.Time) AlertMsg {
	return AlertMsg{
		Ns:          ns,
		Host:        host,
		Measurement: measurement,
		Level:       level,
		Tags:        tags,
		Value:       value,
		Time:        time,
	}
}
