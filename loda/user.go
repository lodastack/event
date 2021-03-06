package loda

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/requests"

	"github.com/lodastack/log"
)

var (
	// UserMap save user infomation.
	UserMap = map[string]User{}
	userMu  sync.RWMutex
)

func init() {
	go clearUserMap()
}

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

// GetUserMobile return user mobile info.
func GetUserMobile(username []string) map[string]string {

	usermobile := make(map[string]string)

	userMap, _ := GetUsers(username)
	for _, user := range userMap {
		if user.Mobile == "" {
			continue
		}
		usermobile[user.Username] = user.Mobile
	}
	return usermobile
}

// GetUserSurmary return user info at format username(mobile).
// e.g: return user(mobile) for get user info to notify.
func GetUserSurmary(users []string) []string {
	receiverInfoSplit := make([]string, len(users))
	receiverInfo, err := GetUsers(users)
	if err != nil {
		log.Errorf("GetUsers fail: %s", err.Error())
	}
	var i int
	// keep the same order， map don't keep the order
	for _, username := range users {
		for _, receiver := range receiverInfo {
			if username == receiver.Username {
				receiverInfoSplit[i] = fmt.Sprintf("%s(%s)", receiver.Username, receiver.Mobile)
				i++
				break
			}
		}
	}
	return receiverInfoSplit[:i]
}

// GetUsers return user information list of usernames.
func GetUsers(usernames []string) (map[string]User, error) {
	userMap := make(map[string]User, len(usernames))
	userMu.RLock()
	usernameUnknown := make([]string, len(usernames))
	var i int
	for _, username := range usernames {
		if user, ok := UserMap[username]; !ok {
			usernameUnknown[i] = username
			i++
		} else {
			if user.Alert != "disable" {
				userMap[username] = user
			}
		}
	}
	usernameUnknown = usernameUnknown[:i]
	userMu.RUnlock()

	if i != 0 {
		userMu.Lock()
		userMapFromServer, err := getUsersFromServer(usernameUnknown)
		if err != nil || len(usernameUnknown) != len(userMapFromServer) {
			log.Errorf("getUsersFromServer error happen or response unmatch with Request, err: %v, request: %v, resp: %v",
				err, usernameUnknown, userMapFromServer)
		}
		for username, user := range userMapFromServer {
			UserMap[username] = user
			if user.Alert != "disable" {
				userMap[username] = user
			}
		}
		userMu.Unlock()
	}

	return userMap, nil
}

// User define the property the user should have.
type User struct {
	Username string `json:"username"`
	Mobile   string `json:"mobile"`
	Alert    string `json:"alert,omitempty"`
}

// RespUser is response from regsitry to query user.
type respUser struct {
	HTTPStatus int             `json:"httpstatus"`
	Data       map[string]User `json:"data"`
}

func getUsersFromServer(usernames []string) (map[string]User, error) {
	var respUser respUser
	url := fmt.Sprintf("%s/api/v1/event/user/list?usernames=%s", config.GetConfig().Reg.Link, strings.Join(usernames, ","))

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("http status code: %d", resp.Status)
	}
	err = json.Unmarshal(resp.Body, &respUser)
	if err != nil {
		log.Errorf("get group error: %s", err.Error())
		return nil, err
	}
	return respUser.Data, nil
}
