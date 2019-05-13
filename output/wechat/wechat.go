package wechat

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/models"
	"github.com/lodastack/log"
)

const (
	timeFormat = "2006-01-02 15:04:05"

	multi = "convergence"
)

func SendWechat(notifyData models.NotifyData) error {
	var title string
	if notifyData.Msg == "" {
		title = fmt.Sprintf("报警:%s  %s", notifyData.AlarmName, notifyData.Level)
	}

	content := genWechatContent(notifyData)

	if len(notifyData.Receivers) == 0 {
		log.Errorf("invalid Users: %v", notifyData.Receivers)
		return nil
	}

	users := strings.Join(notifyData.Receivers, "|")
	if _, err := os.Stat(config.GetConfig().Wechat.Script); err != nil {
		log.Errorf("not found send wechat script: %s", config.GetConfig().Wechat.Script)
		return err
	}
	if out, err := exec.Command("/bin/bash", config.GetConfig().Wechat.Script, users, title, content).Output(); err != nil {
		log.Errorf("run wechat script error: %s, output: %s", err.Error(), string(out))
	}
	return nil
}

func genWechatContent(notifyData models.NotifyData) string {
	if notifyData.Msg != "" {
		return notifyData.Msg
	}
	var ipDesc string
	if notifyData.IP != "" {
		ipDesc = "ip: " + notifyData.IP + "\n"
	}

	var tagDescribe string
	for k, v := range notifyData.Tags {
		tagDescribe += k + ":\t  " + v + "\n"
	}
	if len(notifyData.Tags) > 1 {
		tagDescribe = tagDescribe[:len(tagDescribe)-1]
	}

	return fmt.Sprintf("内容:\nmeasurement:  %s\nns: %s\n%s%s\nvalue: %.2f \ntime: %v",
		notifyData.Measurement,
		notifyData.Ns,
		ipDesc,
		tagDescribe,
		notifyData.Value,
		notifyData.Time.Format(timeFormat))
}
