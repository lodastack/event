package mail

import (
	"bytes"
	"fmt"
	"net/smtp"
	"runtime"
	"strings"
	"text/template"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/models"
)

const (
	timeFormat     = "2006-01-02 15:04:05"
	defaultSubject = "monitor alert"
	OK             = "OK"

	multi = "convergence"
)

var mailSuffix, mailSubject string

func SendEMail(alertMsg models.AlertMsg) error {
	var subject string

	revieve := make([]string, len(alertMsg.Users))
	mailSuffix = config.GetConfig().Mail.MailSuffix
	for index, username := range alertMsg.Users {
		revieve[index] = username + mailSuffix
	}

	var msg string
	if alertMsg.Msg != "" {
		subject = alertMsg.AlarmName
		msg = alertMsg.AlarmName + " " + multi + "\n" +
			"Ns:  " + alertMsg.Ns + "\nalert too many\n" +
			alertMsg.Msg
		msg = strings.Replace(msg, "\n", "</br>", -1)
	} else {
		subject = fmt.Sprintf("%s   %s   is  %s",
			alertMsg.Host, alertMsg.Measurement, alertMsg.Level)
		msg = genMailContent(alertMsg)
	}
	subject = config.GetConfig().Mail.SubjectPrefix + " " + subject

	return SendMail(config.GetConfig().Mail.Host,
		config.GetConfig().Mail.Port,
		config.GetConfig().Mail.User,
		config.GetConfig().Mail.Pwd,
		config.GetConfig().Mail.User+mailSuffix,
		revieve, []string{""},
		subject,
		msg)
}

func genMailContent(alertMsg models.AlertMsg) string {
	var tagDescribe string
	for k, v := range alertMsg.Tags {
		tagDescribe += k + ":\t" + v + "</br>"
	}
	tagDescribe = tagDescribe[:len(tagDescribe)-5]

	var levelColor string
	if alertMsg.Level == OK {
		levelColor = "green"
	} else {
		levelColor = "red"
	}
	status := fmt.Sprintf("<font style=\"color:%s\">%s</font>", levelColor, alertMsg.Level)

	return fmt.Sprintf("%s\t%s</br></br>ns: %s</br>ip: %s</br>%s </br>value: %.2f </br></br>time: %v",
		alertMsg.AlarmName,
		status,
		alertMsg.Ns,
		alertMsg.IP,
		tagDescribe,
		alertMsg.Value,
		alertMsg.Time.Format(timeFormat))
}

func catchPanic(err *error, functionName string) {
	if r := recover(); r != nil {
		fmt.Printf("%s : PANIC Defered : %v\n", functionName, r)

		// Capture the stack trace
		buf := make([]byte, 10000)
		runtime.Stack(buf, false)

		fmt.Printf("%s : Stack Trace : %s", functionName, string(buf))

		if err != nil {
			*err = fmt.Errorf("%v", r)
		}
	} else if err != nil && *err != nil {
		fmt.Printf("%s : ERROR : %v\n", functionName, *err)

		// Capture the stack trace
		buf := make([]byte, 10000)
		runtime.Stack(buf, false)

		fmt.Printf("%s : Stack Trace : %s", functionName, string(buf))
	}
}

func SendMail(host string, port int, userName string, password string, from string, to []string, cc []string, subject string, message string) (err error) {
	defer catchPanic(&err, "SendEmail")
	parameters := struct {
		From    string
		To      string
		Cc      string
		Subject string
		Message string
	}{
		userName,
		strings.Join([]string(to), ","),
		strings.Join([]string(cc), ","),
		subject,
		message,
	}

	buffer := new(bytes.Buffer)

	template := template.Must(template.New("emailTemplate").Parse(emailScript()))
	template.Execute(buffer, &parameters)

	auth := LoginAuth(userName, password)

	err = sendMail(
		fmt.Sprintf("%s:%d", host, port),
		auth,
		from,
		to,
		buffer.Bytes())

	return err
}

func emailScript() (script string) {
	return `From: {{.From}}
To: {{.To}}
Cc: {{.Cc}}
Subject: {{.Subject}}
MIME-version: 1.0
Content-Type: text/html; charset="UTF-8"

{{.Message}}`
}

type loginAuth struct {
	username, password string
}

// loginAuth returns an Auth that implements the LOGIN authentication
// mechanism as defined in RFC 4616.
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	command := string(fromServer)
	command = strings.TrimSpace(command)
	command = strings.TrimSuffix(command, ":")
	command = strings.ToLower(command)

	if more {
		if command == "username" {
			return []byte(fmt.Sprintf("%s", a.username)), nil
		} else if command == "password" {
			return []byte(fmt.Sprintf("%s", a.password)), nil
		} else {
			// We've already sent everything.
			return nil, fmt.Errorf("unexpected server challenge: %s", command)
		}
	}
	return nil, nil
}
