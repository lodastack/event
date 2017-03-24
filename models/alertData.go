package models

import (
	"time"
)

var (
	TagHost = "host"
)

type EventData struct {
	ID       string        `json:"id"`
	Message  string        `json:"message"`
	Details  string        `json:"-"`
	Time     time.Time     `json:"time"`
	Duration time.Duration `json:"duration"`
	Level    string        `json:"level"`
	Data     Result        `json:"data"`

	Ns string `json:"_"`
}

type Result struct {
	Series   Rows
	Messages []*Message
	Err      error
}

type Message struct {
	Level string `json:"level"`
	Text  string `json:"text"`
}

type Rows []*Row

type Row struct {
	Name    string            `json:"name,omitempty"`
	Tags    map[string]string `json:"tags,omitempty"`
	Columns []string          `json:"columns,omitempty"`
	Values  [][]interface{}   `json:"values,omitempty"`
}

func (e *EventData) HasData() bool {
	if len(e.Data.Series) == 0 {
		return false
	}
	return true
}

func (e *EventData) Host() (string, bool) {
	if !e.HasData() {
		return "", false
	}

	host, ok := e.Data.Series[0].Tags[TagHost]
	return host, ok
}
