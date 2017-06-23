package loda

import (
	"strconv"

	m "github.com/lodastack/models"
)

type Alarm struct {
	AlarmData    m.Alarm
	BlockStep    int
	MaxStackTime int
}

var (
	defaultBlockStep = 5  // unit: minute   TODO
	maxBlockTime     = 60 // unit: minute
)

func newAlarm(alarm m.Alarm) *Alarm {
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
