package work

import (
	"strings"

	"github.com/lodastack/event/models"
	"github.com/lodastack/log"
	m "github.com/lodastack/models"

	"github.com/coreos/etcd/client"
)

// ClusterStatus is a simplified interface for manager status from Cluster.
type ClusterStatus interface {
	// GetStatus return interface to query status by ns/alarm/host/level from local data.
	GetStatusFromLocal(ns string) LocalStatusInf

	// QueryStatus query status to cluster.
	GetStatusFromCluster(ns, alarmVersion, host string) (models.Status, error)

	// SetStatus set the status to cluster.
	SetStatus(ns string, alarm m.Alarm, host string, newStatus models.Status) error

	// ClearStatus clear status by ns/alarmVersion/host.
	ClearStatus(ns, alarmVersion, host string) error

	// GenGlobalStatus update global NsStatus according to cluster.
	GenGlobalStatus() error
}

// LocalStatusInf is a simplified interface for manager status from local.
type LocalStatusInf interface {
	// GetStatusList return status list by level(OK, CRITICAL...).
	GetStatusList(level string) []models.Status

	// GetNsStatus return WalkResult reveal the ns status.
	GetNsStatus() models.WalkResult

	// GetAlarmStatus return WalkResult reveal the alarm status.
	GetAlarmStatus() models.WalkResult

	// GetNotOkHost return WalkResult reveal the not ok status.
	GetNotOkHost() models.WalkResult
}

// NewStatus return StatusInf.
func NewStatus(c Cluster) ClusterStatus {
	return &status{c: c}
}

// Status is an instance of the status package.
type status struct {
	c Cluster
}

// GetStatusFromLocal read the status data and return GetStatusInf from local data.
func (s *status) GetStatusFromLocal(ns string) LocalStatusInf {
	status := models.GetNsStatusFromGlobal(ns)
	return &status
}

// GetStatusFromCluster query status to cluster.
func (s *status) GetStatusFromCluster(ns, alarmVersion, host string) (models.Status, error) {
	path := HostStatusKey(ns, alarmVersion, host)
	rep, err := s.c.RecursiveGet(path)
	if err != nil {
		return models.Status{}, err
	}
	return models.NewStatusByString(rep.Node.Value)
}

// SetStatus set the status to cluster.
func (s *status) SetStatus(ns string, alarm m.Alarm, host string, newStatus models.Status) error {
	statusPath := HostStatusKey(ns, alarm.Version, host)
	statusString, _ := newStatus.String()
	return s.c.Set(statusPath, statusString, &client.SetOptions{})
}

// ClearStatus clear status by ns/alarmVersion/host.
func (s *status) ClearStatus(ns, alarmVersion, host string) error {
	models.StatusMu.Lock()
	defer models.StatusMu.Unlock()
	alarmsStatus, ok := models.StatusData[models.NS(ns)]
	if !ok {
		return nil
	}
	for _alarmVersion, _hostStatus := range alarmsStatus {
		if alarmVersion != "" && models.ALARM(alarmVersion) != _alarmVersion {
			continue
		}
		for _host := range _hostStatus {
			if host != "" && models.HOST(host) != _host {
				continue
			}
			delete(models.StatusData[models.NS(ns)][models.ALARM(alarmVersion)], models.HOST(host))

			statusStatusPath := AbsPath(HostStatusKey(ns, string(_alarmVersion), string(_host)))
			hostStatusPath := AbsPath(HostKey(ns, string(_alarmVersion), string(_host)))
			if err := s.c.RemoveDir(statusStatusPath); err != nil && !strings.Contains(err.Error(), "Key not found") {
				log.Errorf("del status dir %s fail: %s", statusStatusPath, err.Error())
			}
			if err := s.c.RemoveDir(hostStatusPath); err != nil && !strings.Contains(err.Error(), "Key not found") {
				if !strings.Contains(err.Error(), "Key not found") {
					log.Errorf("del host dir %s fail: %s", hostStatusPath, err.Error())
				}
			}
		}
	}

	return nil
}

// GenGlobalStatus read status data from cluster, and update the global NsStatus.
func (s *status) GenGlobalStatus() error {
	data := make(models.NsStatus)
	if err := s.genNsStatus(&data); err != nil {
		log.Errorf("HandleStatus get ns fail: %s", err.Error())
		return err
	}

	if err := s.genAlarmStatusForNs(&data); err != nil {
		log.Errorf("HandleStatus get alarm fail: %s", err.Error())
		return err
	}

	if err := s.genHostStatusForAlarm(&data); err != nil {
		log.Errorf("HandleStatus get alarm fail: %s", err.Error())
		return err
	}
	models.StatusMu.Lock()
	models.StatusData = data
	models.StatusMu.Unlock()
	return nil
}

func (s *status) genNsStatus(nsStatus *models.NsStatus) error {
	rep, err := s.c.RecursiveGet("")
	if err != nil {
		log.Errorf("work HandleStatus get root fail: %s", err.Error())
		return err
	}

	for _, node := range rep.Node.Nodes {
		(*nsStatus)[models.NS(ReadEtcdLastSplit(node.Key))] = make(map[models.ALARM]models.HostStatus)
	}
	return nil
}

func (s *status) genAlarmStatusForNs(nsStatus *models.NsStatus) error {
	for ns := range *nsStatus {
		rep, err := s.c.RecursiveGet(string(ns))
		if err != nil {
			log.Errorf("work HandleStatus get ns %s fail: %s", ns, err.Error())
			continue
			// return err ?
		}
		for _, node := range rep.Node.Nodes {
			alarmVersion := ReadEtcdLastSplit(node.Key)
			(*nsStatus)[ns][models.ALARM(alarmVersion)] = models.HostStatus{}
		}
	}
	return nil
}

func (s *status) genHostStatusForAlarm(nsStatus *models.NsStatus) error {
	for ns := range *nsStatus {
		for alarmVersion := range (*nsStatus)[ns] {
			rep, err := s.c.RecursiveGet(StatusDir(string(ns), string(alarmVersion)))
			if err != nil {
				log.Errorf("work HandleStatus get ns %s alarm %s fail: %s", ns, alarmVersion, err.Error())
				continue
			}
			for _, node := range rep.Node.Nodes {
				host := ReadEtcdLastSplit(node.Key)
				if node.Value != "" {
					hostStatus, err := models.NewStatusByString(node.Value)
					if err != nil {
						log.Errorf("unmarshal ns %s alarm %s status fail: %s", ns, alarmVersion, err.Error())
					}
					(*nsStatus)[ns][alarmVersion][models.HOST(host)] = hostStatus
				}
			}
		}
	}
	return nil
}
