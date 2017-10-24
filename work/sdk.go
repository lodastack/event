package work

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/loda"
	m "github.com/lodastack/models"
	"github.com/lodastack/sdk-go"
)

func sendToSDK(ms []m.Metric) error {
	data, err := json.Marshal(ms)
	if err != nil {
		return err
	}
	return sdk.Post(config.GetConfig().Com.EventLogNs, data)
}

// SdkLog log the event/status via sdk.
type SdkLog struct {
}

var (
	sdkLog SdkLog

	eventMetricName  = "alert"
	statusMetricName = "statusv2"
)

// newMetric make []m.Metric with the param.
func (s *SdkLog) newMetric(name, ns, measurement, alarmLevel, host, status string, receives []string, value float64) []m.Metric {
	ms := make([]m.Metric, 1)
	receiverList := loda.GetUserInfo(receives)

	ms[0] = m.Metric{
		Timestamp: time.Now().Unix(),
		Tags: map[string]string{
			"alertname":   name,
			"host":        host,
			"measurement": measurement,
			"level":       alarmLevel,
			"ns":          ns,
			"status":      status,
			"to":          strings.Join(receiverList, "\\,")},
		Value: fmt.Sprintf("%.2f", value),
	}
	return ms
}

// setLastTime set last time string to ms.
func (s *SdkLog) setNameToMetric(ms []m.Metric, name string) {
	for i := range ms {
		ms[i].Name = name
	}
}

// setLastTime set last time string to ms.
// lastTimeStr is string(unit: second).
func (s *SdkLog) setLastTime(ms []m.Metric, lastTimeStr string) {
	for i := range ms {
		ms[i].Tags["last"] = lastTimeStr
	}
}

// Event log the event via sdk.(v1) It is used when output a alarm.
func (s *SdkLog) Event(name, ns, measurement, alarmLevel, host, level string, receives []string, value float64) error {
	ms := s.newMetric(name, ns, measurement, alarmLevel, host, level, receives, value)
	s.setNameToMetric(ms, eventMetricName)
	return sendToSDK(ms)
}

// NewStatus log a new status via sdkl.(v2)  maybe the event is the first alarm of this ns/alarm/host.
func (s *SdkLog) NewStatus(name, ns, measurement, alarmLevel, host, status string, receives []string, value float64) error {
	ms := s.newMetric(name, ns, measurement, alarmLevel, host, status, receives, value)
	s.setNameToMetric(ms, statusMetricName)
	s.setLastTime(ms, "0")
	return sendToSDK(ms)
}

// StatusChange log a status change event via sdk.
func (s *SdkLog) StatusChange(name, ns, measurement, alarmLevel, host, status string, receives []string, value float64, statusStartTime time.Time) error {
	ms := s.newMetric(name, ns, measurement, alarmLevel, host, status, receives, value)
	s.setNameToMetric(ms, statusMetricName)
	lastTime := strconv.Itoa(int(time.Now().Sub(statusStartTime) / time.Second))
	s.setLastTime(ms, lastTime)
	return sendToSDK(ms)
}
