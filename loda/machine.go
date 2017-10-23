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

// RespMachineSearch is the response from registry to search machine resource.
type respMachineSearch struct {
	Status int                            `json:"httpstatus"`
	Data   map[string][]map[string]string `json:"data"`
}

// RespMachineGet is the response from registry to get machine resource.
type respMachineGet struct {
	Status int                 `json:"httpstatus"`
	Data   []map[string]string `json:"data"`
}

// getMachineURI is the URI to get machine resource.
const getMachineURI = "/api/v1/event/resource?ns=%s&type=machine"

var (
	// offlineMachines keep the offline machine.
	offlineMachines map[string]map[string]bool
	// get offline machine interval
	offlineMachineInterval time.Duration = 60

	// Machines save all machine resource, hostname is the key of map.
	Machines  map[string]map[string]string
	machineMu sync.RWMutex
)

// UpdateOffMachineLoop update all offline machine to offlineMachines.
func UpdateOffMachineLoop() {
	var err error
	machineMu.Lock()
	offlineMachines, err = getOfflineMachines()
	machineMu.Unlock()
	if err != nil {
		log.Errorf("get offline machine err: %s", err.Error())
	}

	getMachines := func() {
		machines, err := allMachine()
		if err != nil {
			log.Errorf("get offline machine err: %s", err.Error())
		} else {
			machineMu.Lock()
			Machines = machines
			machineMu.Unlock()
		}

		machineStatus, err := getOfflineMachines()
		if err != nil {
			log.Errorf("get offline machine err: %s", err.Error())
		} else {
			machineMu.Lock()
			offlineMachines = machineStatus
			machineMu.Unlock()
		}
	}

	getMachines()
	go func() {
		c := time.Tick(offlineMachineInterval * time.Second)
		for {
			select {
			case <-c:
				getMachines()
			}
		}
	}()
}

// allMachine return all ns and its machine resource from registry.
func allMachine() (map[string]map[string]string, error) {
	allNs, err := allNS()
	if err != nil {
		return nil, err
	}
	allMachine := make(map[string]map[string]string, 0)
	for _, ns := range allNs {
		if allMachine[ns], err = oneNsMachine(ns); err != nil {
			return nil, err
		}
	}
	return allMachine, nil
}

// oneNsMachine return machines of one ns.
func oneNsMachine(ns string) (map[string]string, error) {
	var respMachineData respMachineGet
	var machineIps map[string]string
	url := fmt.Sprintf("%s"+getMachineURI, config.GetConfig().Reg.Link, ns)

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return machineIps, err
	}
	if resp.Status != 200 {
		return machineIps, fmt.Errorf("http status code: %d", resp.Status)
	}
	err = json.Unmarshal(resp.Body, &respMachineData)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return machineIps, err
	}

	machineIps = make(map[string]string, len(respMachineData.Data))
	for _, machine := range respMachineData.Data {
		ip, _ := machine["ip"]
		hostname, _ := machine["hostname"]
		if ip != "" && hostname != "" {
			machineIps[hostname] = ip
		}
	}

	return machineIps, nil
}

// getOfflineMachines return the map of offline hostname from registry.
func getOfflineMachines() (map[string]map[string]bool, error) {
	var respSearchResp respMachineSearch
	var offlineMachine map[string]map[string]bool
	url := fmt.Sprintf("%s/api/v1/event/resource/search?ns=%s&type=%s&k=%s&v=%s",
		config.GetConfig().Reg.Link, "loda", "machine", "status", "offline")

	resp, err := requests.Get(url)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return offlineMachine, err
	}
	if resp.Status != 200 {
		return offlineMachine, fmt.Errorf("http status code: %d", resp.Status)
	}
	err = json.Unmarshal(resp.Body, &respSearchResp)
	if err != nil {
		log.Errorf("get all ns error: %s", err.Error())
		return offlineMachine, err
	}

	offlineMachine = make(map[string]map[string]bool, len(respSearchResp.Data))
	for ns, machines := range respSearchResp.Data {
		offlineMachine[ns] = make(map[string]bool, len(machines))
		for _, machine := range machines {
			if hostname, ok := machine["hostname"]; ok && hostname != "" {
				offlineMachine[ns][hostname] = true
			}
		}
	}

	return offlineMachine, nil
}

// IsOfflineMachine return a machine is offline status or not.
// Return false if the hostname is not found.
func IsOfflineMachine(ns, hostname string) bool {
	machineMu.RLock()
	defer machineMu.RUnlock()
	if _, ok := offlineMachines[ns]; !ok {
		return false
	}
	if _, ok := offlineMachines[ns][hostname]; !ok {
		return false
	}
	return true
}

// MachineIP return ip by hostname.
// Return offline status if the hostname is not found.
func MachineIP(ns, hostname string) (string, bool) {
	machineMu.RLock()
	defer machineMu.RUnlock()
	nsMachine, ok := Machines[ns]
	if !ok || len(nsMachine) == 0 {
		return "", false
	}
	ip, ok := nsMachine[hostname]
	return ip, ok
}
