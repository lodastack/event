package loda

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lodastack/event/common"
	"github.com/lodastack/event/config"
	"github.com/lodastack/event/requests"

	"github.com/lodastack/log"
)

type User struct {
	Username string `json:"username"`
	Mobile   string `json:"mobile"`
}

type ResUser struct {
	HttpStatus int             `json:"httpstatus"`
	Data       map[string]User `json:"data"`
}

func GetUserMobile(usernames []string) ([]string, error) {
	var resUser ResUser
	var mobiles []string
	url := fmt.Sprintf("%s/api/v1/event/user/list?usernames=%s", config.GetConfig().Reg.Link, strings.Join(usernames, ","))

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return mobiles, err
	}

	if resp.Status != 200 {
		return mobiles, fmt.Errorf("http status code: %d", resp.Status)
	}
	err = json.Unmarshal(resp.Body, &resUser)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return mobiles, err
	}
	mobiles = make([]string, len(resUser.Data))
	var i int
	for _, user := range resUser.Data {
		mobiles[i] = user.Mobile
		i++
	}
	mobiles = common.RemoveDuplicateAndEmpty(mobiles)

	return mobiles[:], nil
}
