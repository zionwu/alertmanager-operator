package alertmanager

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Sirupsen/logrus"

	alertapi "github.com/zionwu/alertmanager-operator/api"
	"github.com/zionwu/alertmanager-operator/client"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	"github.com/zionwu/alertmanager-operator/util"
)

type Synchronizer interface {
	Run(<-chan struct{})
}

type synchronizer struct {
	cfg     *alertapi.Config
	mclient client.Interface
}

func NewSynchronizer(cfg *alertapi.Config, mclient client.Interface) Synchronizer {
	return &synchronizer{
		cfg:     cfg,
		mclient: mclient,
	}
}

func (s *synchronizer) Run(stopc <-chan struct{}) {

	tickChan := time.NewTicker(time.Second * 30).C

	for {
		select {
		case <-tickChan:
			apiAlerts, err := util.GetActiveAlertListFromAlertManager(s.cfg.ManagerUrl)
			if err != nil {
				logrus.Errorf("Error while getting alert list from alertmanager: %v", err)
			} else {
				al, err := s.mclient.MonitoringV1().Alerts(metav1.NamespaceAll).List(metav1.ListOptions{})
				if err != nil {
					logrus.Errorf("Error while geting alert CRD list: %v", err)
				} else {
					alertList := al.(*v1beta1.AlertList)
					for _, alert := range alertList.Items {
						if alert.State == v1beta1.AlertStateDisabled {
							continue
						}

						state, a := util.GetState(alert, apiAlerts)
						needUpdate := false

						//only take ation when the state is not the same
						if state != alert.State {

							//if the origin state is silenced, and current state is active, then need to remove the silence rule
							if alert.State == v1beta1.AlertStateSuppressed && state == v1beta1.AlertStateEnabled {
								util.RemoveSilence(s.cfg.ManagerUrl, alert)
							}

							alert.State = state
							needUpdate = true
						}

						if state == v1beta1.AlertStateSuppressed || state == v1beta1.AlertStateActive {
							if !alert.StartsAt.Equal(a.StartsAt) {
								alert.StartsAt = a.StartsAt
								needUpdate = true
							}

							if !alert.EndsAt.Equal(a.EndsAt) {
								alert.EndsAt = a.EndsAt
								needUpdate = true
							}
						} else {
							alert.StartsAt = time.Time{}
							alert.EndsAt = time.Time{}
						}

						if needUpdate {
							_, err := s.mclient.MonitoringV1().Alerts(alert.Namespace).Update(alert)
							if err != nil {
								logrus.Errorf("Error occurred while syn alert state and time: %v", err)
							}
						}

					}
				}
			}

		case <-stopc:
			return
		}
	}

}
