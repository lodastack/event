package loda

import (
	"strconv"

	m "github.com/lodastack/models"
)

type Alarm struct {
	AlarmData    m.Alarm
	BlockStack   int
	MaxStackTime int
}

var (
	defaultBlockStack = 5  // unit: minute   TODO
	maxBlockTime      = 60 // unit: minute
)

func newAlarm(alarm m.Alarm) *Alarm {
	output := Alarm{AlarmData: alarm}

	var err error
	if output.BlockStack, err = strconv.Atoi(alarm.BlockStack); err != nil || output.BlockStack < 1 {
		output.BlockStack = defaultBlockStack
	}
	if output.MaxStackTime, err = strconv.Atoi(alarm.MaxStackTime); err != nil || output.MaxStackTime < 10 {
		output.MaxStackTime = maxBlockTime
	}
	return &output
}
