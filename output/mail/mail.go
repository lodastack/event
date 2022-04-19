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
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
)

const (
	timeFormat     = "2006-01-02 15:04:05"
	defaultSubject = "monitor alert"
	OK             = "OK"

	multi = "convergence"
)

var mailSuffix, mailSubject string

func SendEMail(notifyData models.NotifyData) error {
	var revieve []string
	mailSuffix = config.GetConfig().Mail.MailSuffix

	var allowedUsers []string
	rsmap, err := loda.GetUsers(notifyData.Receivers)
	if err != nil {
		log.Errorf("mail send get users failed: %s", err)
		allowedUsers = notifyData.Receivers
		rsmap = nil
	}
	for _, u := range rsmap {
		allowedUsers = append(allowedUsers, u.Username)
	}

	for _, username := range allowedUsers {
		u := strings.TrimSpace(username)
		if u != "" {
			revieve = append(revieve, u+mailSuffix)
		}
	}

	var addPng bool
	pngBase64, err := getPngBase64(notifyData)
	if err == nil && len(pngBase64) != 0 {
		addPng = true
	} else {
		log.Errorf("getPngBase64 fail, msg: %+v, err: %+v, length: %d", notifyData, err, len(pngBase64))
	}

	// deploy case
	if notifyData.Msg != "" {
		addPng = false
	}

	return SendMail(config.GetConfig().Mail.Host,
		config.GetConfig().Mail.Port,
		config.GetConfig().Mail.User,
		config.GetConfig().Mail.Pwd,
		config.GetConfig().Mail.From,
		revieve,
		[]string{""},
		genMailSubject(notifyData),
		genMailContent(notifyData),
		addPng, pngBase64,
	)
}

func genMailSubject(notifyData models.NotifyData) string {
	if notifyData.Msg != "" {
		return notifyData.AlarmName
	}
	return fmt.Sprintf("%s %s   %s   is  %s",
		config.GetConfig().Mail.SubjectPrefix, notifyData.Host, notifyData.Measurement, notifyData.Level)
}

func genMailContent(notifyData models.NotifyData) string {
	if notifyData.Msg != "" {
		return strings.Replace(notifyData.Msg, "\n", "</br>", -1)
	}
	var tagDescribe string
	if len(notifyData.Tags) > 0 {
		for k, v := range notifyData.Tags {
			tagDescribe += k + ":\t" + v + "</br>"
		}
		tagDescribe = tagDescribe[:len(tagDescribe)-5]
	}

	var levelColor string
	if notifyData.Level == OK {
		levelColor = "green"
	} else {
		levelColor = "red"
	}
	status := fmt.Sprintf("<font style=\"color:%s\">%s</font>", levelColor, notifyData.Level)

	var ipDesc string
	if notifyData.IP != "" {
		ipDesc = "</br>ip: " + notifyData.IP
	}
	return fmt.Sprintf("%s\t%s</br></br>ns: %s%s</br>%s </br>value: %.2f </br></br>time: %v",
		notifyData.AlarmName,
		status,
		notifyData.Ns,
		ipDesc,
		tagDescribe,
		notifyData.Value,
		notifyData.Time.Format(timeFormat))
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
		from,
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

	if err != nil {
		log.Errorf("send mail: to [%v] subject [%s] %s", to, subject, err)
	}
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
