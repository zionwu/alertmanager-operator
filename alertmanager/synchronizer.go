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
						if alert.State == v1beta1.AlertStateInactive {
							continue
						}

						state := util.GetState(alert, apiAlerts)

						//only take ation when the state is not the same
						if state != alert.State {

							//if the origin state is silenced, and current state is active, then need to remove the silence rule
							if alert.State == v1beta1.AlertStateSilenced && state == v1beta1.AlertStateActive {
								util.RemoveSilence(s.cfg.ManagerUrl, alert)
							}

							alert.State = state
							s.mclient.MonitoringV1().Alerts(alert.Namespace).Update(alert)
						}
					}
				}
			}

		case <-stopc:
			return
		}
	}

}
