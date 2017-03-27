package outputs

import (
	"github.com/lodastack/event/models"
	"github.com/lodastack/event/output/mail"
	"github.com/lodastack/event/output/sms"
)

func init() {
	Handlers = make(map[string]HandleFunc)
	Handlers["mail"] = mail.SendEMail
	Handlers["sms"] = sms.SendSMS
}

type HandleFunc func(alertMsg models.AlertMsg) error

var Handlers map[string]HandleFunc
