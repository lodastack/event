package work

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
	"github.com/lodastack/log"
	m "github.com/lodastack/models"
	"github.com/lodastack/sdk-go"
)

func getRecieverInfo(revievers []string) []string {
	receiverInfoSplit := make([]string, len(revievers))
	receiverInfo, err := loda.GetUsers(revievers...)
	if err != nil {
		log.Errorf("GetUsers fail: %s", err.Error())
	}
	var i int
	for _, receiver := range receiverInfo {
		receiverInfoSplit[i] = fmt.Sprintf("%s(%s)", receiver.Username, receiver.Mobile)
		i++
	}
	return receiverInfoSplit[:i]
}

func logAlarm(name, ns, measurement, host, level string, reveivers []string, value float64) error {
	ms := make([]m.Metric, 1)
	receiverList := getRecieverInfo(reveivers)

	ms[0] = m.Metric{
		Name:      "alert",
		Timestamp: time.Now().Unix(),
		Tags: map[string]string{
			"alertname":   name,
			"host":        host,
			"measurement": measurement,
			"ns":          ns,
			"level":       level,
			"to":          strings.Join(receiverList, "\\,")},
		Value: fmt.Sprintf("%.2f", value),
	}

	data, err := json.Marshal(ms)
	if err != nil {
		return err
	}
	return sdk.Post(config.GetConfig().Com.EventLogNs, data)
}

func logStatus(status models.Status) error {
	ms := make([]m.Metric, 1)
	ms[0] = m.Metric{
		Name:      "status",
		Timestamp: time.Now().Unix(),
		Tags: map[string]string{
			"measurement": status.Measurement,
			"ns":          status.Ns,
			"host":        status.Host,
			"":            ""},
		// Value: fmt.Sprintf("%.2f", value),
	}
	if status.Level == OK {
		ms[0].Value = 0
	} else {
		ms[0].Value = 1
	}

	data, err := json.Marshal(ms)
	if err != nil {
		return err
	}
	return sdk.Post(config.GetConfig().Com.EventLogNs, data)
}

func logNewStatus(status models.Status) error {
	return logStatus(status)
}

func logOneStatus(status models.Status) error {
	return logStatus(status)
}
