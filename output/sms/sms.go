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
	var tagDescribe string
	for k, v := range alertMsg.Tags {
		tagDescribe += k + "\t:  " + v + "\r\n"
	}
	if len(alertMsg.Tags) > 1 {
		tagDescribe = tagDescribe[:len(tagDescribe)-2]
	}

	var msg string
	if alertMsg.Msg != "" {
		msg = alertMsg.AlarmName + " " + multi + "\n" +
			"Ns:  " + alertMsg.Ns + "\nalert too many\n" +
			alertMsg.Msg
		msg = strings.Replace(msg, "\n", "\r\n", -1)
	} else {
		msg = fmt.Sprintf("%s  %s\r\n%s  %s  %s\r\nns: %s\r\n%s \r\nvalue: %.2f \r\ntime: %v",
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

	mobiles, err := loda.GetUserMobile(alertMsg.Users)
	fmt.Println("mobile number:", mobiles, err)
	for _, mobile := range mobiles {
		go func(mobile string) {
			if mobile == "" || len(mobile) != 11 {
				log.Error("invalid mobile: %s", mobile)
				return
			}
			_, err := requests.PostWithHeader(config.GetConfig().Sms.Url,
				map[string]string{"mobile": mobile, "content": "监控报警:" + msg}, []byte{},
				map[string]string{"authToken": config.GetConfig().Sms.Token}, 10)
			if err != nil {
				log.Error("send sms fail: %s", err.Error())
			}
		}(mobile)
	}
	return nil
}
