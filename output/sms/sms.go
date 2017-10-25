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
)

func SendSMS(notifyData models.NotifyData) error {
	mobiles := loda.GetUserMobile(notifyData.Receivers)
	content := genSmsContent(notifyData)

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

func genSmsContent(notifyData models.NotifyData) string {
	if notifyData.Msg != "" {
		return strings.Replace(notifyData.Msg, "\n", "\r\n", -1)
	}

	var tagDescribe string
	for k, v := range notifyData.Tags {
		tagDescribe += k + "\t:  " + v + "\r\n"
	}
	if len(notifyData.Tags) > 1 {
		tagDescribe = tagDescribe[:len(tagDescribe)-2]
	}
	return fmt.Sprintf("%s  %s\r\n%s  %s  %s\r\nns: %s\r\n%s \r\nvalue: %.2f \r\ntime: %v",
		notifyData.AlarmName,
		notifyData.Level,
		notifyData.Host,
		notifyData.Measurement,
		notifyData.Expression,

		notifyData.Ns,
		tagDescribe,
		notifyData.Value,
		notifyData.Time.Format(timeFormat))
}
