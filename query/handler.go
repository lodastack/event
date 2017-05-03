package query

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/lodastack/event/models"
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
	succResp(resp, "OK", eventData)
	resp.WriteHeader(200)
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

	allStatus, err := worker.HandleStatus(ns)
	if err != nil {
		log.Errorf("Work handle status error: %s.", err.Error())
		errResp(resp, http.StatusInternalServerError, "handle status error")
		return
	}

	switch level {
	case "ns":
		succResp(resp, "OK", allStatus.CheckByNs())
	case "alarm":
		succResp(resp, "OK", allStatus.CheckByAlarm(ns))
	default:
		succResp(resp, "OK", allStatus.CheckByHost(ns))
	}

	resp.WriteHeader(200)
}
