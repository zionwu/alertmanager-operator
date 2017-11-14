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
	ev1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"

	"k8s.io/client-go/tools/cache"
)

type daemonSetWatcher struct {
	informer cache.SharedIndexInformer
	cfg      *api.Config
	alert    *v1beta1.Alert
	stopc    chan struct{}
}

func newDaemonSetWatcher(alert *v1beta1.Alert, kclient kubernetes.Interface, cfg *api.Config) Watcher {
	rclient := kclient.Extensions().RESTClient()

	plw := cache.NewListWatchFromClient(rclient, "daemonsets", alert.Namespace, fields.OneTermEqualSelector(k8sapi.ObjectNameField, alert.TargetID))
	informer := cache.NewSharedIndexInformer(plw, &ev1beta1.DaemonSet{}, resyncPeriod, cache.Indexers{})
	stopc := make(chan struct{})

	daemonSetWatcher := &daemonSetWatcher{
		informer: informer,
		alert:    alert,
		cfg:      cfg,
		stopc:    stopc,
	}

	return daemonSetWatcher
}

func (w *daemonSetWatcher) Watch() {
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAdd,
		DeleteFunc: w.handleDelete,
		UpdateFunc: w.handleUpdate,
	})

	go w.informer.Run(w.stopc)
	//<-w.stopc
}

func (w *daemonSetWatcher) Stop() {
	close(w.stopc)
}

func (w *daemonSetWatcher) UpdateAlert(alert *v1beta1.Alert) {
	w.alert = alert
}

func (w *daemonSetWatcher) handleAdd(obj interface{}) {

}

func (w *daemonSetWatcher) handleDelete(obj interface{}) {

}

func (w *daemonSetWatcher) handleUpdate(oldObj, curObj interface{}) {

	//will not check status if the state is inactive
	if w.alert.State == v1beta1.AlertStateInactive {
		return
	}

	oldDaemonSet, err := convertToDaemonSet(oldObj)
	if err != nil {
		logrus.Error("converting to DaemonSet object failed")
		return
	}

	curDaemonSet, err := convertToDaemonSet(curObj)
	if err != nil {
		logrus.Error("converting to DaemonSet object failed")
		return
	}

	if curDaemonSet.GetResourceVersion() != oldDaemonSet.GetResourceVersion() {
		logrus.Infof("different version, will not check daemonset status")
		return
	}

	if w.alert.DaemonSetRule == nil {
		logrus.Errorf("The daemonset rules for %s should not be empty", w.alert.Name)
		return
	}

	if w.alert.DaemonSetRule.UnavailablePercentage == 0 {
		return
	}

	availableThreshold := (100 - w.alert.DaemonSetRule.UnavailablePercentage) * (curDaemonSet.Status.DesiredNumberScheduled) / 100

	if curDaemonSet.Status.NumberAvailable <= availableThreshold {
		logrus.Infof("%s is firing", w.alert.Description)
		err = util.SendAlert(w.cfg.ManagerUrl, w.alert)
		if err != nil {
			logrus.Errorf("Error while sending alert: %v", err)
		}
	} else {
		logrus.Debugf("%s is ok", w.alert.Description)
	}
}

func convertToDaemonSet(o interface{}) (*ev1beta1.DaemonSet, error) {
	ss, isDaemonSet := o.(*ev1beta1.DaemonSet)
	if !isDaemonSet {
		deletedState, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("Received unexpected object: %v", o)
		}
		ss, ok = deletedState.Obj.(*ev1beta1.DaemonSet)
		if !ok {
			return nil, fmt.Errorf("DeletedFinalStateUnknown contained non-Pod object: %v", deletedState.Obj)
		}
	}

	return ss, nil
}
