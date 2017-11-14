package watch

import (
	"time"

	"k8s.io/client-go/kubernetes"

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
	case "statefulset":
		return newNodeWatcher(alert, kclient, cfg)
	case "daemonset":
		return newDaemonSetWatcher(alert, kclient, cfg)
	}
	return nil
}
