package loda

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/lodastack/event/config"
	"github.com/lodastack/event/requests"

	"github.com/lodastack/log"
)

type MachineStatus map[string]bool

type MachineResp struct {
	Status int                            `json:"httpstatus"`
	Data   map[string][]map[string]string `json:"data"`
}

var offlineMachines map[string]MachineStatus
var mu sync.RWMutex
var offlineMachineInterfal time.Duration = 30

func UpdateOffMachineLoop() {
	var err error
	mu.Lock()
	offlineMachines, err = OfflineMachines()
	mu.Unlock()
	if err != nil {
		log.Errorf("get offline machine err: %s", err.Error())
	}
	go func() {
		c := time.Tick(offlineMachineInterfal * time.Second)
		for {
			select {
			case <-c:
				machineStatus, err := OfflineMachines()
				if err != nil {
					log.Errorf("get offline machine err: %s", err.Error())
				} else {
					mu.Lock()
					offlineMachines = machineStatus
					mu.Unlock()
				}
			}
		}
	}()
}

func IsMachineOffline(ns, hostname string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if _, ok := offlineMachines[ns]; !ok {
		return false
	}
	if _, ok := offlineMachines[ns][hostname]; !ok {
		return false
	}
	return true
}

func OfflineMachines() (map[string]MachineStatus, error) {
	var searchResp MachineResp
	var res map[string]MachineStatus
	url := fmt.Sprintf("%s/api/v1/event/resource/search?ns=%s&type=%s&k=%s&v=%s",
		config.GetConfig().Reg.Link, "loda", "machine", "status", "offline")

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return res, err
	}

	if resp.Status != 200 {
		return res, fmt.Errorf("http status code: %d", resp.Status)
	}

	err = json.Unmarshal(resp.Body, &searchResp)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return res, err
	}
	res = make(map[string]MachineStatus, len(searchResp.Data))
	for ns, machines := range searchResp.Data {
		res[ns] = make(map[string]bool, len(machines))
		for _, machine := range machines {
			if hostname, ok := machine["hostname"]; ok && hostname != "" {
				res[ns][hostname] = true
			}
		}
	}

	return res, nil
}
