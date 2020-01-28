package requests

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/lodastack/log"
)

func PostBytes(url string, data []byte) (*Resp, error) {
	return doPostPain(url, bytes.NewReader(data))
}

func Post(url string, obj interface{}) (*Resp, error) {
	data, err := getReader(obj)
	if err != nil {
		return nil, err
	}
	return doPost(url, data)
}

func doPost(url string, data io.Reader) (*Resp, error) {
	resp, err := http.Post(url, "application/json", data)
	return post(resp, err)
}

func post(resp *http.Response, err error) (*Resp, error) {
	if err != nil {
		return nil, err
	}

	response := new(Resp)
	response.Status = resp.StatusCode
	if body, err := getBytes(resp); err != nil {
		return nil, err
	} else {
		response.Body = body
		return response, nil
	}
}

func doPostPain(url string, data io.Reader) (*Resp, error) {
	resp, err := http.Post(url, "text/plain", data)
	return post(resp, err)
}

func getReader(obj interface{}) (io.Reader, error) {
	if bs, err := json.Marshal(&obj); err != nil {
		return nil, err
	} else {
		rs := bytes.NewReader(bs)
		return rs, nil
	}
}

func PostWithHeader(URL string, queryMap map[string]string, body []byte, header map[string]string, timeout int) ([]byte, error) {
	if timeout == 0 {
		timeout = 5
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	var q string
	for k, v := range queryMap {
		q += k + "=" + url.QueryEscape(v) + "&"
	}

	req, _ := http.NewRequest(http.MethodPost, URL+"?"+q, bytes.NewBuffer(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}

	res, err := client.Do(req)
	if err != nil {
		log.Errorf("post fail: %s", err.Error())
		return nil, err
	}
	defer res.Body.Close()
	resp, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("read response fail: %s", err.Error())
		return nil, err
	}

	if res.StatusCode > 299 {
		log.Errorf("return status not 2xx, request: %s, return body: %s", URL+"?"+q, string(resp))
		return nil, errors.New("response status not OK")
	}

	return resp, nil
}
