package watch

import (
	"fmt"

	"github.com/Sirupsen/logrus"

	"github.com/zionwu/alertmanager-operator/api"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	k8sapi "k8s.io/client-go/pkg/api"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	ev1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type deploymentWatcher struct {
	informer cache.SharedIndexInformer
	cfg      *api.Config
	alert    *v1beta1.Alert
	stopc    chan struct{}
}

func newDeploymentWatcher(alert *v1beta1.Alert, kclient kubernetes.Interface, cfg *api.Config) Watcher {
	rclient := kclient.Core().RESTClient()

	plw := cache.NewListWatchFromClient(rclient, "deployments", alert.Namespace, fields.OneTermEqualSelector(k8sapi.ObjectNameField, alert.TargetID))
	informer := cache.NewSharedIndexInformer(plw, &apiv1.Pod{}, resyncPeriod, cache.Indexers{})
	stopc := make(chan struct{})

	deploymentWatcher := &deploymentWatcher{
		informer: informer,
		alert:    alert,
		cfg:      cfg,
		stopc:    stopc,
	}

	return deploymentWatcher
}

func (w *deploymentWatcher) Watch() {
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAdd,
		DeleteFunc: w.handleDelete,
		UpdateFunc: w.handleUpdate,
	})

	go w.informer.Run(w.stopc)
	//<-w.stopc
}

func (w *deploymentWatcher) Stop() {
	close(w.stopc)
}

func (w *deploymentWatcher) UpdateAlert(alert *v1beta1.Alert) {
	w.alert = alert
}

func (w *deploymentWatcher) handleAdd(obj interface{}) {

}

func (w *deploymentWatcher) handleDelete(obj interface{}) {

}

func (w *deploymentWatcher) handleUpdate(oldObj, curObj interface{}) {
	oldDeployment, err := convertToDeployment(oldObj)
	if err != nil {
		logrus.Info("converting to Deployment object failed")
		return
	}

	curDeployment, err := convertToDeployment(curObj)
	if err != nil {
		logrus.Info("converting to Deployment object failed")
		return
	}

	if curDeployment.GetResourceVersion() != oldDeployment.GetResourceVersion() {
		logrus.Infof("different version, will not check node status")
		return
	}

	availableThreshold := (100 - w.alert.DeploymentRule.UnavailablePercentage) * (*curDeployment.Spec.Replicas) / 100

	if curDeployment.Status.AvailableReplicas <= availableThreshold {
		logrus.Infof("%s is firing", w.alert.Description)
		err = sendAlert(w.cfg.ManagerUrl, w.alert)
		if err != nil {
			logrus.Errorf("Error while sending alert: %v", err)
		}
	}

}

func convertToDeployment(o interface{}) (*ev1beta1.Deployment, error) {
	d, isDeployment := o.(*ev1beta1.Deployment)
	if !isDeployment {
		deletedState, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("Received unexpected object: %v", o)
		}
		d, ok = deletedState.Obj.(*ev1beta1.Deployment)
		if !ok {
			return nil, fmt.Errorf("DeletedFinalStateUnknown contained non-Pod object: %v", deletedState.Obj)
		}
	}

	return d, nil
}
