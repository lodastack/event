package outputs

import (
	"github.com/lodastack/event/models"
	"github.com/lodastack/event/output/mail"
)

func init() {
	Handlers = make(map[string]HandleFunc)
	Handlers["mail"] = mail.SendEMail // TODO
}

type HandleFunc func(alertMsg models.AlertMsg) error

var Handlers map[string]HandleFunc
