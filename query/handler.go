package query

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/models"
	o "github.com/lodastack/event/output"
	m "github.com/lodastack/models"

	"github.com/lodastack/log"
)

// @desc get measurement tags from influxdb deps on ns name
// @router /tags [get]
func postDataHandler(resp http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" && req.Method != "POST" {
		errResp(resp, http.StatusMethodNotAllowed, "Get or POST please!")
		return
	}
	params, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		log.Error("parse url error:", err.Error())
		errResp(resp, http.StatusInternalServerError, "parse url error")
		return
	}
	alarmversion := params.Get("version")
	if alarmversion == "" {
		errResp(resp, http.StatusBadRequest, "invalid alarm version")
		return
	}

	versionSplit := strings.Split(alarmversion, m.VersionSep)
	// TODO: check
	// fmt.Printf("http handler  %+v\n", versionSplit)

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorf("Read body fail: %s.", err.Error())
		errResp(resp, http.StatusInternalServerError, "read body fail")
		return
	}

	var eventData models.EventData
	if err = json.Unmarshal(body, &eventData); err != nil {
		log.Errorf("Json unmarshal error: %s.", err.Error())
		errResp(resp, http.StatusInternalServerError, "parse json error")
		return
	}

	ns := versionSplit[0]
	err = worker.HandleEvent(ns, alarmversion, eventData)
	if err != nil {
		log.Errorf("Work handle event error: %s.", err.Error())
		errResp(resp, http.StatusInternalServerError, "handle event error")
		return
	}

	// just return the origin influxdb rs
	resp.Header().Add("Content-Type", "application/json")
	succResp(resp, 200, "OK", eventData)
}

// @router /status [get]
func statusHandler(resp http.ResponseWriter, req *http.Request) {
	params, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		log.Error("parse url error:", err.Error())
		errResp(resp, http.StatusInternalServerError, "parse url error")
		return
	}
	ns := params.Get("ns")
	level := params.Get("level")
	status := params.Get("status")

	statusData := worker.Status.GetStatus(ns)
	if err != nil {
		log.Errorf("Work handle status error: %s.", err.Error())
		errResp(resp, http.StatusInternalServerError, "handle status error")
		return
	}

	switch level {
	case "ns":
		succResp(resp, 200, "OK", statusData.GetNsStatus())
	case "alarm":
		succResp(resp, 200, "OK", statusData.GetAlarmStatus())
	case "host":
		succResp(resp, 200, "OK", statusData.GetNotOkHost())
	default:
		succResp(resp, 200, "OK", statusData.GetStatusList(status))
	}
}

func clearStatusHandler(resp http.ResponseWriter, req *http.Request) {
	params, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		log.Error("parse url error:", err.Error())
		errResp(resp, http.StatusInternalServerError, "parse url error")
		return
	}
	ns := params.Get("ns")
	alarm := params.Get("alarm")
	host := params.Get("host")
	if ns == "" || (alarm == "" && host == "") {
		errResp(resp, http.StatusBadRequest, "invalid param")
		return
	}
	if err := worker.Status.ClearStatus(ns, alarm, host); err != nil {
		log.Errorf("Work ClearBlock %s %s %s error: %s.", ns, alarm, host, err.Error())
		errResp(resp, http.StatusInternalServerError, "handle clear status")
		return
	}
	succResp(resp, 200, "OK", nil)
}

func outputHandler(resp http.ResponseWriter, req *http.Request) {
	params, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		log.Error("parse url error:", err.Error())
		errResp(resp, http.StatusInternalServerError, "parse url error")
		return
	}
	Types := params.Get("types")
	Subject := params.Get("subject")
	Content := params.Get("content")
	groups := params.Get("groups")

	if Types == "" || Content == "" || groups == "" {
		succResp(resp, 400, "param is invalid", nil)
		return
	}

	for _, _type := range strings.Split(Types, ",") {
		handler, ok := o.Handlers[_type]
		if !ok {
			succResp(resp, 400, "type is invalid", nil)
			return
		}

		go func(handler o.HandleFunc, receivers []string) {
			if err := handler(models.AlertMsg{
				Msg:       Content,
				AlarmName: Subject,
				Receivers: receivers}); err != nil {
				log.Error("output fail:", err.Error())
			}
		}(handler, loda.GetGroupUsers(strings.Split(groups, ",")))
	}

	succResp(resp, 200, "OK", nil)
}
