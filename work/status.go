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
	GetStatusFromCluster(ns, alarmVersion, hostname, tagString string) (models.Status, error)

	// SetStatus set the status to cluster.
	SetStatus(ns string, alarm m.Alarm, hostname, tagString string, newStatus models.Status) error

	// ClearStatus clear status by ns/alarmVersion/host.
	ClearStatus(ns, alarmVersion, host, tagString string) error

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

// GetStatusFromCluster query status from cluster.
func (s *status) GetStatusFromCluster(ns, alarmVersion, hostname, tagString string) (models.Status, error) {
	path := StatusKey(ns, alarmVersion, hostname, tagString)
	rep, err := s.c.RecursiveGet(path)
	if err != nil {
		return models.Status{}, err
	}
	return models.NewStatusByString(rep.Node.Value)
}

// SetStatus set the status to cluster.
func (s *status) SetStatus(ns string, alarm m.Alarm, hostname, tagString string, newStatus models.Status) error {
	statusPath := StatusKey(ns, alarm.Version, hostname, tagString)
	statusString, _ := newStatus.String()
	return s.c.Set(statusPath, statusString, &client.SetOptions{})
}

// ClearStatus clear status by ns/alarmVersion/host.
func (s *status) ClearStatus(ns, alarmVersion, hostname, tagString string) error {
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
		for _host, _hostStatus := range _hostStatus {
			if hostname != "" && models.HOST(hostname) != _host {
				continue
			}
			for _tag := range _hostStatus {
				if tagString != "" && models.TAG(tagString) != _tag {
					continue
				}

				delete(models.StatusData[models.NS(ns)][models.ALARM(alarmVersion)][models.HOST(hostname)], models.TAG(tagString))

				tagPath := AbsPath(TagDir(ns, string(_alarmVersion), string(_host), tagString))
				if err := s.c.RemoveDir(tagPath); err != nil && !strings.Contains(err.Error(), "Key not found") {
					log.Errorf("del status dir %s fail: %s", tagPath, err.Error())
				}
			}
		}
	}

	return nil
}

// GenGlobalStatus read status data from cluster, and update the global NsStatus.
func (s *status) GenGlobalStatus() error {
	data := make(models.NsStatus)
	if err := s.genGlobalStatus(&data); err != nil {
		log.Errorf("HandleStatus get ns fail: %s", err.Error())
		return err
	}

	models.StatusMu.Lock()
	models.StatusData = data
	models.StatusMu.Unlock()
	return nil
}

func (s *status) genGlobalStatus(nsStatus *models.NsStatus) error {
	rep, err := s.c.RecursiveGet("")
	if err != nil {
		log.Errorf("work HandleStatus get root fail: %s", err.Error())
		return err
	}

	// ns loop
	for _, nsNode := range rep.Node.Nodes {
		_ns := models.NS(ReadEtcdLastSplit(nsNode.Key))
		(*nsStatus)[_ns] = make(map[models.ALARM]models.HostStatus)
		// ns/alarm loop
		for _, alarmNode := range nsNode.Nodes {
			_alarmVersion := models.ALARM(ReadEtcdLastSplit(alarmNode.Key))
			(*nsStatus)[_ns][_alarmVersion] = models.HostStatus{}
			// ns/alarm/host loop
			for _, hostNode := range alarmNode.Nodes {
				_host := models.HOST(ReadEtcdLastSplit(hostNode.Key))
				(*nsStatus)[_ns][_alarmVersion][_host] = models.TagStatus{}
				// ns/alarm/host/tag loop
				for _, tagNode := range hostNode.Nodes {
					_tagString := models.TAG(ReadEtcdLastSplit(tagNode.Key))
					// read status of ns/alarm/host/tag
					for _, statusOrBlockNode := range tagNode.Nodes {
						if !isStatusPath(statusOrBlockNode.Key) {
							continue
						}
						// read status of ns/alarm/host/tag
						status, err := models.NewStatusByString(statusOrBlockNode.Value)
						if err != nil {
							log.Errorf("unmarshal ns %s alarm %s host %s tag %s status fail: %s", _ns, _alarmVersion, _host, _tagString, err.Error())
							continue
						}
						status.TagString = string(_tagString)
						(*nsStatus)[_ns][_alarmVersion][_host][_tagString] = status
					}
				}
			}
		}
	}
	return nil
}
