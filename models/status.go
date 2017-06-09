package models

import (
	"time"

	"github.com/json-iterator/go"
)

var timeFormat string = "2006-01-02 15:04:05"

type Status struct {
	Name        string
	Measurement string
	Alarm       string
	Host        string
	Ip          string
	Ns          string
	Level       string
	Value       float64
	Tags        map[string]string
	Reciever    []string

	CTime      string
	UTime      string
	LastTime   float64   // unit: min
	CreateTime time.Time `json:"-"`
	UpdateTime time.Time `json:"-"`

	Msg string
}

func NewStatusByString(input string) (Status, error) {
	var status Status
	err := jsoniter.Unmarshal([]byte(input), &status)
	status.CreateTime, _ = time.Parse(timeFormat, status.CTime)
	status.UpdateTime, _ = time.Parse(timeFormat, status.UTime)
	return status, err
}

func (s *Status) String() (string, error) {
	s.CTime = s.CreateTime.Format(timeFormat)
	s.UTime = s.UpdateTime.Format(timeFormat)
	b, err := jsoniter.Marshal(s)
	return string(b), err
}
