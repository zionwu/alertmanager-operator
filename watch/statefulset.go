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
	appv1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"

	"k8s.io/client-go/tools/cache"
)

type statefulSetWatcher struct {
	informer cache.SharedIndexInformer
	cfg      *api.Config
	alert    *v1beta1.Alert
	stopc    chan struct{}
}

func newStatefulSetWatcher(alert *v1beta1.Alert, kclient kubernetes.Interface, cfg *api.Config) Watcher {
	rclient := kclient.Apps().RESTClient()

	plw := cache.NewListWatchFromClient(rclient, "statefulsets", alert.Namespace, fields.OneTermEqualSelector(k8sapi.ObjectNameField, alert.TargetID))
	informer := cache.NewSharedIndexInformer(plw, &appv1beta1.StatefulSet{}, resyncPeriod, cache.Indexers{})
	stopc := make(chan struct{})

	statefulSetWatcher := &statefulSetWatcher{
		informer: informer,
		alert:    alert,
		cfg:      cfg,
		stopc:    stopc,
	}

	return statefulSetWatcher
}

func (w *statefulSetWatcher) Watch() {
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAdd,
		DeleteFunc: w.handleDelete,
		UpdateFunc: w.handleUpdate,
	})

	go w.informer.Run(w.stopc)
	//<-w.stopc
}

func (w *statefulSetWatcher) Stop() {
	close(w.stopc)
}

func (w *statefulSetWatcher) UpdateAlert(alert *v1beta1.Alert) {
	w.alert = alert
}

func (w *statefulSetWatcher) handleAdd(obj interface{}) {

}

func (w *statefulSetWatcher) handleDelete(obj interface{}) {

}

func (w *statefulSetWatcher) handleUpdate(oldObj, curObj interface{}) {

	//will not check status if the state is inactive
	if w.alert.State == v1beta1.AlertStateInactive {
		return
	}

	oldStatefulSet, err := convertToStatefulSet(oldObj)
	if err != nil {
		logrus.Error("converting to StatefulSet object failed")
		return
	}

	curStatefulSet, err := convertToStatefulSet(curObj)
	if err != nil {
		logrus.Error("converting to StatefulSet object failed")
		return
	}

	if curStatefulSet.GetResourceVersion() != oldStatefulSet.GetResourceVersion() {
		logrus.Infof("different version, will not check statefulset status")
		return
	}

	if w.alert.StatefulSetRule == nil {
		logrus.Errorf("The statefulset rules for %s should not be empty", w.alert.Name)
		return
	}

	if w.alert.StatefulSetRule.UnavailablePercentage == 0 {
		return
	}

	availableThreshold := (100 - w.alert.StatefulSetRule.UnavailablePercentage) * (*curStatefulSet.Spec.Replicas) / 100

	if curStatefulSet.Status.ReadyReplicas <= availableThreshold {
		logrus.Infof("%s is firing", w.alert.Description)
		err = util.SendAlert(w.cfg.ManagerUrl, w.alert)
		if err != nil {
			logrus.Errorf("Error while sending alert: %v", err)
		}
	} else {
		logrus.Debugf("%s is ok", w.alert.Description)
	}
}

func convertToStatefulSet(o interface{}) (*appv1beta1.StatefulSet, error) {

	ss, isStatefulSet := o.(*appv1beta1.StatefulSet)
	if !isStatefulSet {
		deletedState, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("Received unexpected object: %v", o)
		}
		ss, ok = deletedState.Obj.(*appv1beta1.StatefulSet)
		if !ok {
			return nil, fmt.Errorf("DeletedFinalStateUnknown contained non-Pod object: %v", deletedState.Obj)
		}
	}

	return ss, nil
}
