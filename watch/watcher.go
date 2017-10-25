package watch

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/prometheus/common/model"

	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sapi "k8s.io/client-go/pkg/api"
)

type Watcher interface {
	Watch()
}

type watcher struct {
	clientset       *kubernetes.Clientset
	mclient         *v1beta1.MonitoringV1Client
	alertClient     v1beta1.AlertInterface
	alertManagerURL string
}

func NewWatcher(clientset *kubernetes.Clientset, mclient *v1beta1.MonitoringV1Client, alertManagerURL string) Watcher {

	//TODO: should not hardcode name space here
	alertClient := mclient.Alerts(k8sapi.NamespaceDefault)
	return &watcher{
		alertClient:     alertClient,
		mclient:         mclient,
		clientset:       clientset,
		alertManagerURL: alertManagerURL,
	}
}

//TODO: look into how to use watch. best pratice for watch.
func (w *watcher) Watch() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			logrus.Info("new round")

			alertList, err := w.alertClient.List(metav1.ListOptions{})
			if err != nil {
				logrus.Errorf("Error while listing alert CRD: %s", err)
			} else {
				for _, alert := range alertList.Items {
					switch alert.Spec.Object {
					case "pod":
						pod, err := w.clientset.CoreV1().Pods(k8sapi.NamespaceDefault).Get(alert.Spec.ObjectID, metav1.GetOptions{})
						if err != nil {
							logrus.Errorf("Error while geting pods: %s", err)
						} else {
							for _, status := range pod.Status.ContainerStatuses {
								if status.State.Running == nil {
									logrus.Infof("%s is firing", alert.Name)
									err = sendAlert(w.alertManagerURL, alert)
									if err != nil {
										logrus.Errorf("Error while sending alert: %v", err)
									}
								} else {
									logrus.Infof("%s is ok", alert.Name)
								}
							}
						}
					}
				}
			}
		}
	}

}

func sendAlert(url string, alert *v1beta1.Alert) error {
	alertList := model.Alerts{}

	a := &model.Alert{}
	a.Labels = map[model.LabelName]model.LabelValue{}
	a.Labels[model.LabelName("environment")] = model.LabelValue(alert.Namespace)
	a.Labels[model.LabelName("alert_id")] = model.LabelValue(alert.Name)

	alertList = append(alertList, a)

	alertData, err := json.Marshal(alertList)
	if err != nil {
		return err
	}

	resp, err := http.Post(url+"/api/alerts", "application/json", bytes.NewBuffer(alertData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logrus.Infof("res: %s", string(res))

	return nil
}
