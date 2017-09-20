package sms

import (
	"fmt"
	"strings"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
	"github.com/lodastack/event/requests"
	"github.com/lodastack/log"
)

const (
	timeFormat = "2006-01-02 15:04:05"

	multi = "convergence"
)

func SendSMS(alertMsg models.AlertMsg) error {
	mobiles := loda.GetUserMobile(alertMsg.Receivers)
	content := genSmsContent(alertMsg)

	for _, mobile := range mobiles {
		go sendSMS(mobile, content)
	}
	return nil
}

func sendSMS(mobile, content string) {
	if mobile == "" || len(mobile) != 11 {
		log.Error("invalid mobile: %s", mobile)
		return
	}
	if _, err := requests.PostWithHeader(config.GetConfig().Sms.Url,
		map[string]string{"mobile": mobile, "content": "监控报警:" + content},
		[]byte{},
		map[string]string{"authToken": config.GetConfig().Sms.Token},
		10); err != nil {
		log.Error("send sms fail: %s", err.Error())
	}
}

func genSmsContent(alertMsg models.AlertMsg) string {
	if alertMsg.Msg != "" {
		return strings.Replace(alertMsg.Msg, "\n", "\r\n", -1)
	}

	var tagDescribe string
	for k, v := range alertMsg.Tags {
		tagDescribe += k + "\t:  " + v + "\r\n"
	}
	if len(alertMsg.Tags) > 1 {
		tagDescribe = tagDescribe[:len(tagDescribe)-2]
	}
	return fmt.Sprintf("%s  %s\r\n%s  %s  %s\r\nns: %s\r\n%s \r\nvalue: %.2f \r\ntime: %v",
		alertMsg.AlarmName,
		alertMsg.Level,
		alertMsg.Host,
		alertMsg.Measurement,
		alertMsg.Expression,

		alertMsg.Ns,
		tagDescribe,
		alertMsg.Value,
		alertMsg.Time.Format(timeFormat))
}
