package wechat

import (
	"fmt"
	"strings"

	"github.com/lodastack/event/config"
	// "github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
	"github.com/lodastack/event/requests"
	"github.com/lodastack/log"
)

const (
	timeFormat = "2006-01-02 15:04:05"

	multi = "convergence"
)

func SendWechat(alertMsg models.AlertMsg) error {
	var tagDescribe string
	for k, v := range alertMsg.Tags {
		tagDescribe += k + ":\t  " + v + "\n"
	}
	if len(alertMsg.Tags) > 1 {
		tagDescribe = tagDescribe[:len(tagDescribe)-1]
	}

	var msg, title string
	if alertMsg.Msg != "" {
		msg = alertMsg.AlarmName + " " + multi + "\n" +
			"Ns:  " + alertMsg.Ns + "\nalert too many\n" +
			alertMsg.Msg

	} else {
		title = fmt.Sprintf("报警:%s  %s", alertMsg.AlarmName, alertMsg.Level)
		msg = fmt.Sprintf("内容:\nmeasurement:  %s\nns: %s\nip: %s\n%s\nvalue: %.2f \ntime: %v",
			alertMsg.Measurement,
			alertMsg.Ns,
			alertMsg.IP,
			tagDescribe,
			alertMsg.Value,
			alertMsg.Time.Format(timeFormat))
	}

	if len(alertMsg.Users) == 0 {
		log.Error("invalid Users: %v", alertMsg.Users)
		return nil
	}
	_, err := requests.PostWithHeader(config.GetConfig().Wechat.Url,
		map[string]string{"account": strings.Join(alertMsg.Users, "|"), "title": title, "content": msg}, []byte{},
		map[string]string{"authToken": config.GetConfig().Wechat.Token}, 10)
	if err != nil {
		log.Error("send Wechat fail: %s", err.Error())
	}

	return nil
}
