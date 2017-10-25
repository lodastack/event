package models

/*
	kapacitor task sent post request when a event happen which body contain the points from influxDB.
	Event will read the event level(OK, CRITICAL, WARNING, FATAL...) and point to genarate notify message.
*/

import (
	"sort"
	// "time"

	// "github.com/influxdata/kapacitor/alert"
	"github.com/influxdata/kapacitor/services/alert"
)

var (
	TagHost  = "host"
	NoneTags = "none"
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

	host, ok := e.Data.Series[0].Tags[TagHost]
	return host, ok
}

func EncodeTags(m map[string]string) []byte {
	// Empty maps marshal to empty bytes.
	if len(m) == 0 {
		return nil
	}

	// Extract keys and determine final size.
	sz := (len(m) * 2) - 1 // separators
	keys := make([]string, 0, len(m))
	for k, v := range m {
		keys = append(keys, k)
		sz += len(k) + len(v)
	}
	sort.Strings(keys)

	// Generate marshaled bytes.
	b := make([]byte, sz)
	buf := b
	for i, k := range keys {
		copy(buf, k)
		buf[len(k)] = '='
		buf = buf[len(k)+1:]

		v := m[k]
		copy(buf, v)
		if i < len(keys)-1 {
			buf[len(v)] = ';'
			buf = buf[len(v)+1:]
		}
	}

	return b
}

// hasSeries check the eventData has point no not.
func (e *EventData) hasSeries() bool {
	if len(e.Data.Series) == 0 {
		return false
	}
	return true
}
