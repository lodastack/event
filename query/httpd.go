package query

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/work"

	"github.com/lodastack/log"
)

var worker *work.Work

type Response struct {
	StatusCode int         `json:"httpstatus"`
	Msg        string      `json:"msg"`
	Data       interface{} `json:"data"`
}

func errResp(resp http.ResponseWriter, status int, msg string) {
	response := Response{
		StatusCode: status,
		Msg:        msg,
		Data:       nil,
	}
	bytes, _ := json.Marshal(&response)
	resp.Header().Add("Content-Type", "application/json")
	resp.WriteHeader(status)
	resp.Write(bytes)
}

func succResp(resp http.ResponseWriter, code int, msg string, data interface{}) {
	if code == 0 {
		code = http.StatusOK
	}
	response := Response{
		StatusCode: http.StatusOK,
		Msg:        msg,
		Data:       data,
	}
	bytes, _ := json.Marshal(&response)
	resp.Header().Add("Content-Type", "application/json")
	resp.WriteHeader(response.StatusCode)
	resp.Write(bytes)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (this *responseWriter) WriteHeader(code int) {
	this.statusCode = code
	this.ResponseWriter.WriteHeader(code)
}

func cors(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set(`Access-Control-Allow-Origin`, origin)
			w.Header().Set(`Access-Control-Allow-Methods`, strings.Join([]string{
				`DELETE`,
				`GET`,
				`OPTIONS`,
				`POST`,
				`PUT`,
			}, ", "))

			w.Header().Set(`Access-Control-Allow-Headers`, strings.Join([]string{
				`Accept`,
				`Accept-Encoding`,
				`Authorization`,
				`Content-Length`,
				`Content-Type`,
				`X-CSRF-Token`,
				`X-HTTP-Method-Override`,
				`AuthToken`,
				`NS`,
				`Resource`,
				`X-Requested-With`,
			}, ", "))
		}

		if r.Method == "OPTIONS" {
			return
		}

		inner.ServeHTTP(w, r)
	})
}

func addHandlers() {
	prefix := "/event"

	http.Handle(prefix+"/post", cors(http.HandlerFunc(postDataHandler)))
	http.Handle(prefix+"/output", cors(http.HandlerFunc(outputHandler)))
	http.Handle(prefix+"/status", cors(http.HandlerFunc(statusHandler)))
	http.Handle(prefix+"/clear/status", cors(http.HandlerFunc(clearStatusHandler)))
}

func Start(work *work.Work) {
	bind := fmt.Sprintf("%s", config.GetConfig().Com.Listen)
	log.Infof("http start on %s!\n", bind)
	worker = work
	addHandlers()

	err := http.ListenAndServe(bind, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "http start failed:\n%s\n", err.Error())
	}
}
