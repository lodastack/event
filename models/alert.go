package models

/*
	kapacitor task sent post request when a event happen which body contain the points from influxDB.
	Event will read the event level(OK, CRITICAL, WARNING, FATAL...) and point to genarate notify message.
*/

import (
	"github.com/influxdata/kapacitor/services/alert"
)

var (
	HostTagName = "host"
)

// EventData is the object in the body of kapacitor alert request.
type EventData struct {
	alert.AlertData

	Ns string `json:"-"` // Ns id added by event, used to genarete notify message.
}

// Host return the hostname in first series tags.
// NOTE: Kapacitor task will group by host, so the hostname is same in series.
//       But the series maybe has no hostname tag if the alarm in loda set cluster alert mode(not group by host).
func (e *EventData) Host() (string, bool) {
	if !e.hasSeries() {
		return "", false
	}

	host, ok := e.Data.Series[0].Tags[HostTagName]
	return host, ok
}

func (e *EventData) Tag() map[string]string {
	if !e.hasSeries() {
		return nil
	}

	return e.Data.Series[0].Tags
}

// hasSeries check the eventData has point no not.
func (e *EventData) hasSeries() bool {
	if len(e.Data.Series) == 0 {
		return false
	}
	return true
}
