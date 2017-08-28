package loda

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	// "github.com/lodastack/event/common"
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

func init() {
	go clearUserMap()
}

func GetUsersFromServer(usernames []string) (map[string]User, error) {
	var resUser ResUser
	url := fmt.Sprintf("%s/api/v1/event/user/list?usernames=%s", config.GetConfig().Reg.Link, strings.Join(usernames, ","))

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("http status code: %d", resp.Status)
	}
	err = json.Unmarshal(resp.Body, &resUser)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return nil, err
	}
	return resUser.Data, nil
}

var userMu sync.RWMutex
var UserMap map[string]User = map[string]User{}

func clearUserMap() {
	c := time.Tick(1 * time.Minute)
	for {
		select {
		case <-c:
			userMu.Lock()
			UserMap = map[string]User{}
			userMu.Unlock()
		}
	}
}

func GetUsers(usernames ...string) (map[string]User, error) {
	userMap := make(map[string]User, len(usernames))
	userMu.RLock()
	usernameUnknown := make([]string, len(usernames))
	var i int
	for _, username := range usernames {
		if user, ok := UserMap[username]; !ok {
			usernameUnknown[i] = username
			i++
		} else {
			userMap[username] = user
		}
	}
	userMu.RUnlock()

	if i != 0 {
		userMu.Lock()
		userMapFromServer, err := GetUsersFromServer(usernameUnknown)
		if err != nil || len(usernameUnknown) != len(userMapFromServer) {
			log.Errorf("GetUsersFromServer error happen or response unmatch with Request, err: %v, request: %v, resp: %v",
				err, usernameUnknown, userMapFromServer)
		}
		for username, user := range userMapFromServer {
			UserMap[username] = user
			userMap[username] = user
		}
		userMu.Unlock()
	}

	return userMap, nil
}

func GetUserMobile(username []string) []string {
	mobiles := make([]string, len(username))
	var i int

	userMap, _ := GetUsers(username...)
	for _, user := range userMap {
		if user.Mobile == "" {
			continue
		}
		mobiles[i] = user.Mobile
		i++
	}
	return mobiles[:i]
}
