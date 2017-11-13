package alertmanager

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Sirupsen/logrus"

	"github.com/prometheus/alertmanager/dispatch"
	alertapi "github.com/zionwu/alertmanager-operator/api"
	"github.com/zionwu/alertmanager-operator/client"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
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

	tickChan := time.NewTicker(time.Second * 10).C

	for {
		select {
		case <-tickChan:
			apiAlerts, err := s.getActiveAlertListFromAlertManager()
			if err != nil {
				logrus.Errorf("Error while getting alert list from alertmanager: %v", err)
			} else {
				al, err := s.mclient.MonitoringV1().Alerts(metav1.NamespaceAll).List(metav1.ListOptions{})
				if err != nil {
					logrus.Errorf("Error while geting alert CRD list: %v", err)
				} else {
					alertList := al.(*v1beta1.AlertList)
					for _, alert := range alertList.Items {
						state := getState(alert, apiAlerts)
						if state != alert.State {
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

func getState(alert *v1beta1.Alert, apiAlerts []*dispatch.APIAlert) string {

	for _, a := range apiAlerts {
		if string(a.Labels["alert_id"]) == alert.Name && string(a.Labels["namespace"]) == alert.Namespace {
			return "active"
		}
	}

	return "inactive"

}

func (s *synchronizer) getActiveAlertListFromAlertManager() ([]*dispatch.APIAlert, error) {

	res := struct {
		Data   []*dispatch.APIAlert `json:"data"`
		Status string               `json:"status"`
	}{}

	req, err := http.NewRequest(http.MethodGet, s.cfg.ManagerUrl+"/api/v1/alerts", nil)
	if err != nil {
		return nil, err
	}
	//q := req.URL.Query()
	//q.Add("filter", fmt.Sprintf("{%s}", filter))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	requestBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(requestBytes, &res); err != nil {
		return nil, err
	}

	return res.Data, nil
}
