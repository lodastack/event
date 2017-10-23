package loda

import (
	"encoding/json"
	"fmt"

	"github.com/lodastack/event/common"
	"github.com/lodastack/event/config"
	"github.com/lodastack/event/requests"

	"github.com/lodastack/log"
)

var (
	lodaDefault = "loda-defaultuser"
)

// GetGroupUsers return users of the groups.
func GetGroupUsers(groups []string) []string {
	recievers := make([]string, 0)
	for _, gname := range groups {
		users, err := getUserOfGroup(gname)
		if err != nil {
			continue
		}
		recievers = append(recievers, users...)
	}
	recievers = common.RemoveDuplicateAndEmpty(recievers)
	if len(recievers) == 0 {
		return nil
	}
	return recievers
}

// Group define the propertys a group should have.
type Group struct {
	GName    string   `json:"gname"`
	Managers []string `json:"managers"`
	Members  []string `json:"members"`
	Items    []string `json:"items"`
}

// responseGroup is the respose of get group from registry.
type responseGroup struct {
	HTTPStatus int   `json:"httpstatus"`
	Data       Group `json:"data"`
}

// getUserByGroup return the user list of the groupname by query regsitry.
func getUserOfGroup(gname string) ([]string, error) {
	var respGroup responseGroup
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
	err = json.Unmarshal(resp.Body, &respGroup)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return users, err
	}
	users = append(users, respGroup.Data.Managers...)
	users = append(users, respGroup.Data.Members...)
	users = common.RemoveDuplicateAndEmpty(users)

	if i, ok := common.ContainString(users, lodaDefault); ok {
		users[i] = users[len(users)-1]
	}

	return users[:], nil
}
