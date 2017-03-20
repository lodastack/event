package loda

import (
	"encoding/json"
	"fmt"

	"github.com/lodastack/event/common"
	"github.com/lodastack/event/config"
	"github.com/lodastack/event/requests"

	"github.com/lodastack/log"
)

type Group struct {
	GName    string   `json:"gname"`
	Managers []string `json:"managers"`
	Members  []string `json:"members"`
	Items    []string `json:"items"`
}

type ResGroup struct {
	HttpStatus int   `json:"httpstatus"`
	Data       Group `json:"data"`
}

var (
	lodaDefault = "loda-defaultuser"
)

func GetUserByGroup(gname string) ([]string, error) {
	var resGroup ResGroup
	var users []string
	url := fmt.Sprintf("%s/api/v1/event/group?gname=%s", config.GetConfig().Reg.Link, gname)

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return users, err
	}

	if resp.Status != 200 {
		return users, fmt.Errorf("http status code: %d", resp.Status)
	}
	err = json.Unmarshal(resp.Body, &resGroup)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return users, err
	}
	users = append(users, resGroup.Data.Managers...)
	users = append(users, resGroup.Data.Members...)
	users = common.RemoveDuplicateAndEmpty(users)

	if i, ok := common.ContainString(users, lodaDefault); ok {
		users[i] = users[len(users)-1]
	}

	return users[:], nil
}
