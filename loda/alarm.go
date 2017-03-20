package loda

import (
	"strconv"
	"time"

	"github.com/lodastack/log"
	m "github.com/lodastack/models"
)

type Alarm struct {
	AlarmData   m.Alarm
	BlockTicker *time.Ticker
	stop        chan bool

	HostBlockPeriod int
	HostBlockTimes  int
	NsBlockPeriod   int
	NsBlockTimes    int
}

var (
	hostPeroidDefault = 5 // TODO
	nsPeroidDefault   = 5
)

func ifAlarmChanged(new, old m.Alarm) bool {
	if new.HostBlockPeriod != old.HostBlockPeriod ||
		new.HostBlockTimes != old.HostBlockTimes ||
		new.NsBlockPeriod != old.NsBlockPeriod ||
		new.NsBlockTimes != old.NsBlockTimes {
		return true
	}
	return false
}

func newAlarm(alarm m.Alarm) *Alarm {
	hostPeroid, err := strconv.Atoi(alarm.HostBlockPeriod)
	if err != nil || hostPeroid == 0 {
		log.Errorf("Strconv host peroid fail or empty, value: %d, error: %v.", hostPeroid, err)
		hostPeroid = hostPeroidDefault
	}
	output := Alarm{
		AlarmData:   alarm,
		BlockTicker: time.NewTicker(time.Duration(hostPeroid) * time.Minute),
		stop:        make(chan bool, 0),
	}

	if output.HostBlockPeriod, err = strconv.Atoi(alarm.HostBlockPeriod); err != nil || output.HostBlockPeriod == 0 {
		log.Errorf("Strconv host peroid fail or empty, value: %d, error: %v.", output.HostBlockPeriod, err)
		output.HostBlockPeriod = hostPeroidDefault
	}
	if output.NsBlockPeriod, err = strconv.Atoi(alarm.NsBlockPeriod); err != nil || output.NsBlockPeriod == 0 {
		log.Errorf("Strconv ns peroid fail or empty, value: %d, error: %v.", output.NsBlockPeriod, err)
		output.NsBlockPeriod = nsPeroidDefault
	}
	if output.NsBlockTimes, err = strconv.Atoi(alarm.NsBlockTimes); err != nil || output.NsBlockTimes == 0 {
		log.Errorf("Strconv ns nsBlockTimes fail or empty, value: %d, error: %v.", output.NsBlockTimes, err)
	}
	if output.HostBlockTimes, err = strconv.Atoi(alarm.HostBlockTimes); err != nil || output.HostBlockTimes == 0 {
		log.Errorf("Strconv ns hostBlockTimes fail or empty, value: %d, error: %v.", output.HostBlockTimes, err)
	}
	return &output
}

func (a *Alarm) Run(tickerChan chan string) {
	for {
		select {
		case <-a.BlockTicker.C:
			tickerChan <- a.AlarmData.Version
			// fmt.Println("Tick at", t)
		case stop := <-a.stop:
			if stop {
				return
			}
		}
	}
}

func (a *Alarm) Stop() {
	a.stop <- true
}

func (a *Alarm) Update(alarmRes m.Alarm, tickerChan chan string) {
	a.Stop()
	a.AlarmData.HostBlockPeriod = alarmRes.HostBlockPeriod
	a.AlarmData.HostBlockTimes = alarmRes.HostBlockTimes
	a.AlarmData.NsBlockPeriod = alarmRes.NsBlockPeriod
	a.AlarmData.NsBlockTimes = alarmRes.NsBlockTimes

	hostPeroid, err := strconv.Atoi(alarmRes.HostBlockPeriod)
	if err != nil || hostPeroid == 0 {
		log.Errorf("Strconv host peroid fail or empty, value: %d, error: %v.", hostPeroid, err)
		hostPeroid = hostPeroidDefault
	}
	a.BlockTicker = time.NewTicker(time.Duration(hostPeroid) * time.Minute)
	go a.Run(tickerChan)
}
