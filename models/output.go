package models

type AlertMsg struct {
	Users []string
	Msg   string
}

func NewAlertMsg(Users []string, msg string) AlertMsg {
	return AlertMsg{Users: Users, Msg: msg}
}
