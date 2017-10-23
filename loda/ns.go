package loda

import (
	"encoding/json"
	"fmt"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/requests"

	"github.com/lodastack/log"
)

// RespNS is response from registry to get ns list.
type respNS struct {
	Status int      `json:"httpstatus"`
	Data   []string `json:"data"`
}

// allNS get and return all ns from registry.
func allNS() ([]string, error) {
	var resNS respNS
	var res []string
	url := fmt.Sprintf("%s/api/v1/event/ns?ns=&format=list", config.GetConfig().Reg.Link)

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return res, err
	}

	if resp.Status == 200 {
		err = json.Unmarshal(resp.Body, &resNS)
		if err != nil {
			log.Errorf("get all ns error: %s", err.Error())
			return res, err
		}
		return resNS.Data, nil
	}
	return res, fmt.Errorf("http status code: %d", resp.Status)
}
