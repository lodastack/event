package models

import "time"

// NotifyData is the notify infomation.
// It will generate different noitify content by different notify type.
type NotifyData struct {
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

// NewAlertMsg genarate NotifyData by alert infomation.
func NewAlertMsg(ns, host, ip, measurement, level string, alarmName string,
	expression string, receivers []string, tags map[string]string, value float64, time time.Time) NotifyData {
	return NotifyData{
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

// NotifyMsg define the noitfy content, method and recieve groups.
type NotifyMsg struct {
	Types   []string `json:"types"`
	Subject string   `json:"subject"`
	Content string   `json:"content"`
	Groups  []string `json:"groups"`
}
