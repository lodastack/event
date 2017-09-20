package mail

import (
	"bytes"
	"fmt"
	"net/smtp"
	"runtime"
	"strings"
	"text/template"

	"github.com/lodastack/log"

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
	revieve := make([]string, len(alertMsg.Receivers))
	mailSuffix = config.GetConfig().Mail.MailSuffix
	for index, username := range alertMsg.Receivers {
		revieve[index] = username + mailSuffix
	}

	var addPng bool
	pngBase64, err := getPngBase64(alertMsg)
	if err == nil && len(pngBase64) != 0 {
		addPng = true
	} else {
		log.Errorf("getPngBase64 fail, msg: %+v, err: %+v, length: %d", alertMsg, err, len(pngBase64))
	}

	return SendMail(config.GetConfig().Mail.Host,
		config.GetConfig().Mail.Port,
		config.GetConfig().Mail.User,
		config.GetConfig().Mail.Pwd,
		config.GetConfig().Mail.User+mailSuffix, revieve, []string{""},
		config.GetConfig().Mail.SubjectPrefix+" "+genMailSubject(alertMsg),
		genMailContent(alertMsg),
		addPng, pngBase64,
	)
}

func genMailSubject(alertMsg models.AlertMsg) string {
	if alertMsg.Msg != "" {
		return alertMsg.AlarmName
	}
	return fmt.Sprintf("%s   %s   is  %s",
		alertMsg.Host, alertMsg.Measurement, alertMsg.Level)
}

func genMailContent(alertMsg models.AlertMsg) string {
	if alertMsg.Msg != "" {
		return strings.Replace(alertMsg.Msg, "\n", "</br>", -1)
	}
	var tagDescribe string
	if len(alertMsg.Tags) > 0 {
		for k, v := range alertMsg.Tags {
			tagDescribe += k + ":\t" + v + "</br>"
		}
		tagDescribe = tagDescribe[:len(tagDescribe)-5]
	}

	var levelColor string
	if alertMsg.Level == OK {
		levelColor = "green"
	} else {
		levelColor = "red"
	}
	status := fmt.Sprintf("<font style=\"color:%s\">%s</font>", levelColor, alertMsg.Level)

	var ipDesc string
	if alertMsg.IP != "" {
		ipDesc = "</br>ip: " + alertMsg.IP
	}
	return fmt.Sprintf("%s\t%s</br></br>ns: %s%s</br>%s </br>value: %.2f </br></br>time: %v",
		alertMsg.AlarmName,
		status,
		alertMsg.Ns,
		ipDesc,
		tagDescribe,
		alertMsg.Value,
		alertMsg.Time.Format(timeFormat))
}

func catchPanic(err *error, functionName string) {
	if r := recover(); r != nil {
		log.Errorf("sand mail fail, %s : PANIC Defered : %v\n", functionName, r)

		// Capture the stack trace
		buf := make([]byte, 10000)
		runtime.Stack(buf, false)

		log.Errorf("sand mail fail, %s : Stack Trace : %s", functionName, string(buf))

		if err != nil {
			*err = fmt.Errorf("%v", r)
		}
	} else if err != nil && *err != nil {
		log.Errorf("sand mail fail, %s : ERROR : %v\n", functionName, *err)

		// Capture the stack trace
		buf := make([]byte, 10000)
		runtime.Stack(buf, false)

		log.Errorf("sand mail fail, %s : Stack Trace : %s", functionName, string(buf))
	}
}

func SendMail(host string, port int, userName string, password string, from string, to []string, cc []string, subject string, message string, addPng bool, pngBase64 []byte) (err error) {
	defer catchPanic(&err, "SendEmail")
	parameters := struct {
		From      string
		To        string
		Cc        string
		Subject   string
		Message   string
		PngBase64 []byte
	}{
		userName,
		strings.Join([]string(to), ","),
		strings.Join([]string(cc), ","),
		subject,
		message,
		pngBase64,
	}

	buffer := new(bytes.Buffer)
	boundaryTag := "boundary_loda"
	template := template.Must(template.New("emailTemplate").Parse(emailScript(addPng, boundaryTag)))
	template.Execute(buffer, &parameters)
	if addPng {
		buffer.Write(parameters.PngBase64)
		buffer.WriteString("\r\n--" + boundaryTag + "--")
	}
	auth := LoginAuth(userName, password)

	err = sendMail(
		fmt.Sprintf("%s:%d", host, port),
		auth,
		from,
		to,
		buffer.Bytes())

	return err
}

func emailScript(boundary bool, boundaryTag string) (script string) {
	if boundary {
		return fmt.Sprintf(`From: {{.From}}
To: {{.To}}
Cc: {{.Cc}}
Subject: {{.Subject}}
Content-Type: multipart/related; boundary=%s

--%s
Content-Type: text/html; charset=UTF-8


{{.Message}}
<br>  <img src="cid:img1">
--%s
Content-Type: image/png; name="test.png"
Content-ID: <img1>
Content-Disposition: inline;filename=test.png
Content-Transfer-Encoding: base64

`, boundaryTag, boundaryTag, boundaryTag)
	}
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
