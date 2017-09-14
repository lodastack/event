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
	var title string
	if alertMsg.Msg == "" {
		title = fmt.Sprintf("报警:%s  %s", alertMsg.AlarmName, alertMsg.Level)
	}

	content := genWechatContent(alertMsg)

	if len(alertMsg.Receivers) == 0 {
		log.Error("invalid Users: %v", alertMsg.Receivers)
		return nil
	}
	_, err := requests.PostWithHeader(config.GetConfig().Wechat.Url,
		map[string]string{"account": strings.Join(alertMsg.Receivers, "|"), "title": title, "content": content}, []byte{},
		map[string]string{"authToken": config.GetConfig().Wechat.Token}, 10)
	if err != nil {
		log.Error("send Wechat fail: %s", err.Error())
	}

	return nil
}

func genWechatContent(alertMsg models.AlertMsg) string {
	if alertMsg.Msg != "" {
		return alertMsg.Msg
	}
	var ipDesc string
	if alertMsg.IP != "" {
		ipDesc = "ip: " + alertMsg.IP + "\n"
	}

	var tagDescribe string
	for k, v := range alertMsg.Tags {
		tagDescribe += k + ":\t  " + v + "\n"
	}
	if len(alertMsg.Tags) > 1 {
		tagDescribe = tagDescribe[:len(tagDescribe)-1]
	}

	return fmt.Sprintf("内容:\nmeasurement:  %s\nns: %s\n%s%s\nvalue: %.2f \ntime: %v",
		alertMsg.Measurement,
		alertMsg.Ns,
		ipDesc,
		tagDescribe,
		alertMsg.Value,
		alertMsg.Time.Format(timeFormat))
}
