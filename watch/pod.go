package watch

import (
	"github.com/zionwu/alertmanager-operator/api"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
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

	set := map[string]string{}
	set["name"] = alert.Name
	fieldSeletor := fields.SelectorFromSet(set)

	plw := cache.NewListWatchFromClient(rclient, "pods", alert.Namespace, fieldSeletor)
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

}
