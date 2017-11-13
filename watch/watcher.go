package watch

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/common/model"
	"github.com/zionwu/alertmanager-operator/api"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
)

const resyncPeriod = 1 * time.Minute

type Watcher interface {
	Watch()
	Stop()
	UpdateAlert(*v1beta1.Alert)
}

func NewWatcher(alert *v1beta1.Alert, kclient kubernetes.Interface, cfg *api.Config) Watcher {

	switch alert.TargetType {
	case "pod":
		return newPodWatcher(alert, kclient, cfg)
	case "deployment":
		return newDeploymentWatcher(alert, kclient, cfg)
	case "node":
		return newNodeWatcher(alert, kclient, cfg)
	case "statefulSet":
		return newNodeWatcher(alert, kclient, cfg)
	case "daemonSet":
		return newDaemonSetWatcher(alert, kclient, cfg)
	}
	return nil
}

func sendAlert(url string, alert *v1beta1.Alert) error {
	alertList := model.Alerts{}

	a := &model.Alert{}
	a.Labels = map[model.LabelName]model.LabelValue{}
	a.Labels[model.LabelName("namespace")] = model.LabelValue(alert.Namespace)
	a.Labels[model.LabelName("alert_id")] = model.LabelValue(alert.Name)
	a.Labels[model.LabelName("severity")] = model.LabelValue(alert.Severity)
	a.Labels[model.LabelName("target_id")] = model.LabelValue(alert.TargetID)
	a.Labels[model.LabelName("target_type")] = model.LabelValue(alert.TargetType)
	a.Labels[model.LabelName("description")] = model.LabelValue(alert.Description)

	a.Annotations = map[model.LabelName]model.LabelValue{}
	a.Annotations[model.LabelName("description")] = model.LabelValue(alert.Description)

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
