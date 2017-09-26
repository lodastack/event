package status

import (
	"strings"

	"github.com/lodastack/event/models"
	"github.com/lodastack/event/work/cluster"
	"github.com/lodastack/log"
	m "github.com/lodastack/models"

	"github.com/coreos/etcd/client"
)

const (
	// OK status
	OK = "OK"
)

// StatusInf is the package exposed interface.
type StatusInf interface {
	// GetStatus return interface to query status by ns/alarm/host/level from local data.
	GetStatusFromLocal(ns string) GetStatusInf

	// QueryStatus query status to cluster.
	GetStatusFromCluster(ns, alarmVersion, host string) (models.Status, error)

	// SetStatus set the status to cluster.
	SetStatus(ns string, alarm m.Alarm, host string, newStatus models.Status) error

	// ClearStatus clear status by ns/alarmVersion/host.
	ClearStatus(ns, alarmVersion, host string) error

	// GenGlobalStatus update global NsStatus according to cluster.
	GenGlobalStatus() error
}

// NewStatus return StatusInf.
func NewStatus(c cluster.ClusterInf) StatusInf {
	return &Status{c: c}
}

// Status is the struct has StatusInf method.
type Status struct {
	c cluster.ClusterInf
}

// GetStatus read the status data and return GetStatusInf from local data.
func (s *Status) GetStatusFromLocal(ns string) GetStatusInf {
	return &StatusData{
		data: getNsStatusFromGlobal(NS(ns)),
	}
}

// QueryStatus query status to cluster..
func (s *Status) GetStatusFromCluster(ns, alarmVersion, host string) (models.Status, error) {
	path := cluster.HostStatusKey(ns, alarmVersion, host)
	rep, err := s.c.RecursiveGet(path)
	if err != nil {
		return models.Status{}, err
	}
	return models.NewStatusByString(rep.Node.Value)
}

// SetStatus set the status to cluster.
func (s *Status) SetStatus(ns string, alarm m.Alarm, host string, newStatus models.Status) error {
	statusPath := ns + "/" + alarm.Version + "/" + cluster.AlarmStatusPath + "/" + host
	statusString, _ := newStatus.String()
	return s.c.Set(statusPath, statusString, &client.SetOptions{})
}

// ClearStatus clear status by ns/alarmVersion/host.
func (s *Status) ClearStatus(ns, alarmVersion, host string) error {
	mu.Lock()
	defer mu.Unlock()
	alarmsStatus, ok := statusData[NS(ns)]
	if !ok {
		return nil
	}
	for _alarmVersion, _hostStatus := range alarmsStatus {
		if alarmVersion != "" && ALARM(alarmVersion) != _alarmVersion {
			continue
		}
		for _host := range _hostStatus {
			if host != "" && HOST(host) != _host {
				continue
			}
			delete(statusData[NS(ns)][ALARM(alarmVersion)], HOST(host))

			statusStatusPath := cluster.EtcdPrefix + "/" + ns + "/" + string(_alarmVersion) + "/" + cluster.AlarmStatusPath + "/" + string(_host)
			hostStatusPath := cluster.EtcdPrefix + "/" + ns + "/" + string(_alarmVersion) + "/" + cluster.AlarmHostPath + "/" + string(_host)
			if err := s.c.DeleteDir(statusStatusPath); err != nil && !strings.Contains(err.Error(), "Key not found") {
				log.Errorf("del status dir %s fail: %s", statusStatusPath, err.Error())
			}
			if err := s.c.DeleteDir(hostStatusPath); err != nil && !strings.Contains(err.Error(), "Key not found") {
				if !strings.Contains(err.Error(), "Key not found") {
					log.Errorf("del host dir %s fail: %s", hostStatusPath, err.Error())
				}
			}
		}
	}

	return nil
}

// GenGlobalStatus read status data from cluster, and update the global NsStatus.
func (s *Status) GenGlobalStatus() error {
	data := make(NsStatus)
	if err := s.genNsStatus(&data); err != nil {
		log.Error("HandleStatus get ns fail: %s", err.Error())
		return err
	}

	if err := s.genAlarmStatusToNs(&data); err != nil {
		log.Error("HandleStatus get alarm fail: %s", err.Error())
		return err
	}

	if err := s.genHostStatusToAlarm(&data); err != nil {
		log.Error("HandleStatus get alarm fail: %s", err.Error())
		return err
	}
	mu.Lock()
	statusData = data
	mu.Unlock()
	return nil
}

func (s *Status) genNsStatus(status *NsStatus) error {
	rep, err := s.c.RecursiveGet("")
	if err != nil {
		log.Errorf("work HandleStatus get root fail: %s", err.Error())
		return err
	}

	for _, node := range rep.Node.Nodes {
		(*status)[NS(cluster.ReadEtcdLastSplit(node.Key))] = make(map[ALARM]HostStatus)
	}
	return nil
}

func (s *Status) genAlarmStatusToNs(status *NsStatus) error {
	for ns := range *status {
		rep, err := s.c.RecursiveGet(string(ns))
		if err != nil {
			log.Errorf("work HandleStatus get ns %s fail: %s", ns, err.Error())
			continue
			// return err ?
		}
		for _, node := range rep.Node.Nodes {
			alarmVersion := cluster.ReadEtcdLastSplit(node.Key)
			(*status)[ns][ALARM(alarmVersion)] = HostStatus{}
		}
	}
	return nil
}

func (s *Status) genHostStatusToAlarm(status *NsStatus) error {
	for ns := range *status {
		for alarmVersion := range (*status)[ns] {
			rep, err := s.c.RecursiveGet(string(ns) + "/" + string(alarmVersion) + "/" + cluster.AlarmStatusPath)
			if err != nil {
				log.Errorf("work HandleStatus get ns %s alarm %s fail: %s", ns, alarmVersion, err.Error())
				continue
				// return err ?
			}
			for _, node := range rep.Node.Nodes {
				host := cluster.ReadEtcdLastSplit(node.Key)
				if node.Value != "" {
					hostStatus, err := models.NewStatusByString(node.Value)
					if err != nil {
						log.Errorf("unmarshal ns %s alarm %s status fail: %s", ns, alarmVersion, err.Error())
					}
					(*status)[ns][alarmVersion][HOST(host)] = hostStatus
				}
			}
		}
	}
	return nil
}

// GetStatusInf is the interface to query status.
type GetStatusInf interface {
	// GetStatusList return status list by level(OK, CRITICAL...).
	GetStatusList(level string) []models.Status

	// GetNsStatus return WalkResult reveal the ns status.
	GetNsStatus() WalkResult

	// GetAlarmStatus return WalkResult reveal the alarm status.
	GetAlarmStatus() WalkResult

	// GetNotOkHost return WalkResult reveal the not ok status.
	GetNotOkHost() WalkResult
}

// StatusData has GetStatusInf interface to query status from its data.
type StatusData struct {
	data NsStatus
}

// GetStatusList return status list by level(OK, CRITICAL...).
func (s *StatusData) GetStatusList(level string) []models.Status {
	return s.data.getStatusList(level)
}

// GetNsStatus return WalkResult reveal the ns status.
func (s *StatusData) GetNsStatus() WalkResult {
	return s.data.getNsStatus()
}

// GetAlarmStatus return WalkResult reveal the alarm status.
func (s *StatusData) GetAlarmStatus() WalkResult {
	return s.data.getAlarmStatus()
}

// GetNotOkHost return WalkResult reveal the not ok status.
func (s *StatusData) GetNotOkHost() WalkResult {
	return s.data.getNotOkHost()
}
