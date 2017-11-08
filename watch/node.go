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
	"k8s.io/client-go/tools/cache"
)

type nodeWatcher struct {
	informer cache.SharedIndexInformer
	cfg      *api.Config
	alert    *v1beta1.Alert
	stopc    chan struct{}
}

func newNodeWatcher(alert *v1beta1.Alert, kclient kubernetes.Interface, cfg *api.Config) Watcher {
	rclient := kclient.Core().RESTClient()

	plw := cache.NewListWatchFromClient(rclient, "nodes", alert.Namespace, fields.OneTermEqualSelector(k8sapi.ObjectNameField, alert.TargetID))
	informer := cache.NewSharedIndexInformer(plw, &apiv1.Pod{}, resyncPeriod, cache.Indexers{})
	stopc := make(chan struct{})

	nodeWatcher := &nodeWatcher{
		informer: informer,
		alert:    alert,
		cfg:      cfg,
		stopc:    stopc,
	}

	return nodeWatcher
}

func (w *nodeWatcher) Watch() {
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAdd,
		DeleteFunc: w.handleDelete,
		UpdateFunc: w.handleUpdate,
	})

	go w.informer.Run(w.stopc)
	//<-w.stopc
}

func (w *nodeWatcher) Stop() {
	close(w.stopc)
}

func (w *nodeWatcher) UpdateAlert(alert *v1beta1.Alert) {
	w.alert = alert
}

func (w *nodeWatcher) handleAdd(obj interface{}) {

}

func (w *nodeWatcher) handleDelete(obj interface{}) {

}

func (w *nodeWatcher) handleUpdate(oldObj, curObj interface{}) {
	oldNode, err := convertToNode(oldObj)
	if err != nil {
		logrus.Info("converting to Node object failed")
		return
	}

	curNode, err := convertToNode(curObj)
	if err != nil {
		logrus.Info("converting to Node object failed")
		return
	}

	if curNode.GetResourceVersion() != oldNode.GetResourceVersion() {
		logrus.Infof("different version, will not check node status")
		return
	}

	for _, condition := range curNode.Status.Conditions {
		if w.alert.NodeRule.Condition == string(condition.Type) && string(condition.Status) == "True" {
			sendAlert(w.cfg.ManagerUrl, w.alert)
			break
		}

		if w.alert.NodeRule.Condition == "NotReady" && string(condition.Type) == "Ready" && string(condition.Status) == "False" {
			sendAlert(w.cfg.ManagerUrl, w.alert)
			break
		}

	}
}

func convertToNode(o interface{}) (*apiv1.Node, error) {
	node, isNode := o.(*apiv1.Node)
	if !isNode {
		deletedState, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("Received unexpected object: %v", o)
		}
		node, ok = deletedState.Obj.(*apiv1.Node)
		if !ok {
			return nil, fmt.Errorf("DeletedFinalStateUnknown contained non-Pod object: %v", deletedState.Obj)
		}
	}

	return node, nil
}
