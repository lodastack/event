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

var mailSuffix, mailSubject string

func init() {
	mailSuffix = config.GetConfig().Mail.MailSuffix

	mailSubject = config.GetConfig().Mail.MailSubject
	if mailSubject == "" {
		mailSubject = "monitor alert"
	}
}

// TODO
func SendEMail(alertMsg models.AlertMsg) error {
	revieve := make([]string, len(alertMsg.Users))
	for index, username := range alertMsg.Users {
		revieve[index] = username + mailSuffix
	}
	return SendMail(config.GetConfig().Mail.Host, config.GetConfig().Mail.Port, config.GetConfig().Mail.User, config.GetConfig().Mail.Pwd,
		config.GetConfig().Mail.User+mailSuffix, revieve, []string{""}, mailSubject, alertMsg.Msg)
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
