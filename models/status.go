package models

import (
	"time"

	"github.com/json-iterator/go"
)

var timeFormat string = "2006-01-02 15:04:05"

type Status struct {
	Name         string
	Measurement  string
	Alarm        string
	AlarmVersion string
	Host         string
	Ip           string
	Ns           string
	Level        string
	Value        float64
	Tags         map[string]string
	Reciever     []string

	CTime      string
	UTime      string
	LastTime   time.Duration // unit: second
	CreateTime time.Time     `json:"-"`
	UpdateTime time.Time     `json:"-"`

	Msg string
}

func NewStatusByString(input string) (status Status, err error) {
	if err = jsoniter.Unmarshal([]byte(input), &status); err != nil {
		return
	}

	loc := time.Now().Location()
	status.CreateTime, _ = time.ParseInLocation(timeFormat, status.CTime, loc)
	status.UpdateTime, _ = time.ParseInLocation(timeFormat, status.UTime, loc)
	return status, nil
}

func (s *Status) String() (string, error) {
	s.CTime, s.UTime = s.CreateTime.Format(timeFormat), s.UpdateTime.Format(timeFormat)
	b, err := jsoniter.Marshal(s)
	return string(b), err
}
