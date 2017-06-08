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

type MachineSearch struct {
	Status int                            `json:"httpstatus"`
	Data   map[string][]map[string]string `json:"data"`
}
type MachineGet struct {
	Status int                 `json:"httpstatus"`
	Data   []map[string]string `json:"data"`
}

var offlineMachines map[string]MachineStatus
var machinesIp map[string]string
var mu sync.RWMutex
var offlineMachineInterfal time.Duration = 60

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
				ips, err := MachinesIp()
				if err != nil {
					log.Errorf("get offline machine err: %s", err.Error())
				} else {
					mu.Lock()
					machinesIp = ips
					mu.Unlock()
				}

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

func MachineIp(hostname string) string {
	mu.RLock()
	defer mu.RUnlock()
	ip, _ := machinesIp[hostname]
	return ip
}

func OfflineMachines() (map[string]MachineStatus, error) {
	var searchResp MachineSearch
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

func MachinesIp() (map[string]string, error) {
	var machineData MachineGet
	var machineIps map[string]string
	url := fmt.Sprintf("http://registry.monitor.ifengidc.com/api/v1/event/resource?ns=loda&type=machine")

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return machineIps, err
	}

	if resp.Status != 200 {
		return machineIps, fmt.Errorf("http status code: %d", resp.Status)
	}

	err = json.Unmarshal(resp.Body, &machineData)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return machineIps, err
	}
	machineIps = make(map[string]string, len(machineData.Data))
	for _, machine := range machineData.Data {
		ip, _ := machine["ip"]
		hostname, _ := machine["hostname"]
		if ip != "" && hostname != "" {
			machineIps[hostname] = ip
		}
	}

	return machineIps, nil
}
