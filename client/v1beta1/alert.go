package v1beta1

import (
	"encoding/json"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	AlertsKind       = "Alert"
	AlertName        = "alerts"
	AlertSinguleName = "alert"
)

type AlertsGetter interface {
	Alerts(namespace string) AlertInterface
}

//only used to check if the alert struct implements AlertInterface
var _ AlertInterface = &alerts{}

type AlertInterface interface {
	Create(*Alert) (*Alert, error)
	Get(name string, opts metav1.GetOptions) (*Alert, error)
	Update(*Alert) (*Alert, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (runtime.Object, error)
	//List(opts metav1.ListOptions) (*AlertList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error
}

type alerts struct {
	restClient rest.Interface
	client     *dynamic.ResourceClient
	ns         string
}

func newAlerts(r rest.Interface, c *dynamic.Client, namespace string) *alerts {
	return &alerts{
		r,
		c.Resource(
			&metav1.APIResource{
				Kind:       AlertsKind,
				Name:       AlertName,
				Namespaced: true,
			},
			namespace,
		),
		namespace,
	}
}

func (p *alerts) Create(o *Alert) (*Alert, error) {
	up, err := UnstructuredFromAlert(o)
	if err != nil {
		return nil, err
	}

	up, err = p.client.Create(up)
	if err != nil {
		return nil, err
	}

	return AlertFromUnstructured(up)
}

func (p *alerts) Get(name string, opts metav1.GetOptions) (*Alert, error) {
	obj, err := p.client.Get(name, opts)
	if err != nil {
		return nil, err
	}
	return AlertFromUnstructured(obj)
}

func (p *alerts) Update(o *Alert) (*Alert, error) {
	up, err := UnstructuredFromAlert(o)
	if err != nil {
		return nil, err
	}

	curp, err := p.Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current version for update")
	}
	up.SetResourceVersion(curp.ObjectMeta.ResourceVersion)

	up, err = p.client.Update(up)
	if err != nil {
		return nil, err
	}

	return AlertFromUnstructured(up)
}

func (p *alerts) Delete(name string, options *metav1.DeleteOptions) error {
	return p.client.Delete(name, options)
}

func (p *alerts) List(opts metav1.ListOptions) (runtime.Object, error) {
	//func (p *alerts) List(opts metav1.ListOptions) (*AlertList, error) {
	labelSelector, err := labels.Parse(opts.LabelSelector)
	if err != nil {
		return nil, err
	}

	req := p.restClient.Get().
		Namespace(p.ns).
		Resource(AlertName).
		// VersionedParams(&options, v1.ParameterCodec)
		FieldsSelectorParam(nil).LabelsSelectorParam(labelSelector)

	b, err := req.DoRaw()
	if err != nil {
		return nil, err
	}
	var prom AlertList
	return &prom, json.Unmarshal(b, &prom)
}

func (p *alerts) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	r, err := p.restClient.Get().
		Prefix("watch").
		Namespace(p.ns).
		Resource(AlertName).
		// VersionedParams(&options, v1.ParameterCodec).
		FieldsSelectorParam(nil).
		Stream()
	if err != nil {
		return nil, err
	}
	return watch.NewStreamWatcher(&alertDecoder{
		dec:   json.NewDecoder(r),
		close: r.Close,
	}), nil
}

func (p *alerts) DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error {
	return p.client.DeleteCollection(dopts, lopts)
}

func AlertFromUnstructured(r *unstructured.Unstructured) (*Alert, error) {
	b, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	var p Alert
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	p.TypeMeta.Kind = AlertsKind
	p.TypeMeta.APIVersion = Group + "/" + Version
	return &p, nil
}

func UnstructuredFromAlert(p *Alert) (*unstructured.Unstructured, error) {
	p.TypeMeta.Kind = AlertsKind
	p.TypeMeta.APIVersion = Group + "/" + Version
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var r unstructured.Unstructured
	if err := json.Unmarshal(b, &r.Object); err != nil {
		return nil, err
	}
	return &r, nil
}

type alertDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *alertDecoder) Close() {
	d.close()
}

func (d *alertDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e struct {
		Type   watch.EventType
		Object Alert
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}
