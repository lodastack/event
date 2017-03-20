package work

import (
	"github.com/lodastack/event/common"
	"github.com/lodastack/event/models"
	o "github.com/lodastack/event/output"
)

func send(alertType []string, alertMsg models.AlertMsg) error {
	//
	alertType = append(alertType, "mail")
	alertType = common.RemoveDuplicateAndEmpty(alertType)
	//
	for _, handler := range alertType {
		handlerFunc, ok := o.Handlers[handler]
		if !ok {
			// TODO
			continue
		}
		if err := handlerFunc(alertMsg); err != nil {
			return err
		}
	}
	return nil
}
