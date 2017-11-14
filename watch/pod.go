package watch

import (
	"fmt"

	"github.com/Sirupsen/logrus"

	"github.com/zionwu/alertmanager-operator/api"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	"github.com/zionwu/alertmanager-operator/util"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	k8sapi "k8s.io/client-go/pkg/api"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
)

type podWatcher struct {
	informer cache.SharedIndexInformer
	cfg      *api.Config
	alert    *v1beta1.Alert
	stopc    chan struct{}
}

func newPodWatcher(alert *v1beta1.Alert, kclient kubernetes.Interface, cfg *api.Config) Watcher {
	rclient := kclient.Core().RESTClient()

	plw := cache.NewListWatchFromClient(rclient, "pods", alert.Namespace, fields.OneTermEqualSelector(k8sapi.ObjectNameField, alert.TargetID))
	informer := cache.NewSharedIndexInformer(plw, &apiv1.Pod{}, resyncPeriod, cache.Indexers{})
	stopc := make(chan struct{})

	podWatcher := &podWatcher{
		informer: informer,
		alert:    alert,
		cfg:      cfg,
		stopc:    stopc,
	}

	return podWatcher
}

func (w *podWatcher) Watch() {
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAdd,
		DeleteFunc: w.handleDelete,
		UpdateFunc: w.handleUpdate,
	})

	go w.informer.Run(w.stopc)
	//<-w.stopc
}

func (w *podWatcher) Stop() {
	close(w.stopc)
}

func (w *podWatcher) UpdateAlert(alert *v1beta1.Alert) {
	w.alert = alert
}

func (w *podWatcher) handleAdd(obj interface{}) {

}

func (w *podWatcher) handleDelete(obj interface{}) {

}

func (w *podWatcher) handleUpdate(oldObj, curObj interface{}) {

	//will not check status if the state is inactive
	if w.alert.State == v1beta1.AlertStateInactive {
		return
	}

	oldPod, err := convertToPod(oldObj)
	if err != nil {
		logrus.Error("converting to Node object failed")
		return
	}

	curPod, err := convertToPod(curObj)
	if err != nil {
		logrus.Error("converting to Node object failed")
		return
	}

	if curPod.GetResourceVersion() != oldPod.GetResourceVersion() {
		logrus.Infof("different version, will not check pod status")
		return
	}

	for _, status := range curPod.Status.ContainerStatuses {
		if status.State.Running == nil {
			logrus.Infof("%s is firing", w.alert.Description)
			err = util.SendAlert(w.cfg.ManagerUrl, w.alert)
			if err != nil {
				logrus.Errorf("Error while sending alert: %v", err)
			}
			break
		} else {
			logrus.Debugf("%s is ok", w.alert.Description)
		}
	}
}

func convertToPod(o interface{}) (*apiv1.Pod, error) {
	pod, isPod := o.(*apiv1.Pod)
	if !isPod {
		deletedState, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("Received unexpected object: %v", o)
		}
		pod, ok = deletedState.Obj.(*apiv1.Pod)
		if !ok {
			return nil, fmt.Errorf("DeletedFinalStateUnknown contained non-Pod object: %v", deletedState.Obj)
		}
	}

	return pod, nil
}
