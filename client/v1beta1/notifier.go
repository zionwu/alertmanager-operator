package v1beta1

import (
	"encoding/json"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	NotifiersKind = "Notifier"
	NotifierName  = "notifiers"
)

type NotifiersGetter interface {
	Notifiers(namespace string) NotifierInterface
}

var _ NotifierInterface = &notifiers{}

type NotifierInterface interface {
	Create(*Notifier) (*Notifier, error)
	Get(name string, opts metav1.GetOptions) (*Notifier, error)
	Update(*Notifier) (*Notifier, error)
	Delete(name string, options *metav1.DeleteOptions) error
	//List(opts metav1.ListOptions) (runtime.Object, error)
	List(opts metav1.ListOptions) (*NotifierList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error
}

type notifiers struct {
	restClient rest.Interface
	client     *dynamic.ResourceClient
	ns         string
}

func newNotifiers(r rest.Interface, c *dynamic.Client, namespace string) *notifiers {
	return &notifiers{
		r,
		c.Resource(
			&metav1.APIResource{
				Kind:       NotifiersKind,
				Name:       NotifierName,
				Namespaced: true,
			},
			namespace,
		),
		namespace,
	}
}

func (p *notifiers) Create(o *Notifier) (*Notifier, error) {
	up, err := UnstructuredFromNotifier(o)
	if err != nil {
		return nil, err
	}

	up, err = p.client.Create(up)
	if err != nil {
		return nil, err
	}

	return NotifierFromUnstructured(up)
}

func (p *notifiers) Get(name string, opts metav1.GetOptions) (*Notifier, error) {
	obj, err := p.client.Get(name, opts)
	if err != nil {
		return nil, err
	}
	return NotifierFromUnstructured(obj)
}

func (p *notifiers) Update(o *Notifier) (*Notifier, error) {
	up, err := UnstructuredFromNotifier(o)
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

	return NotifierFromUnstructured(up)
}

func (p *notifiers) Delete(name string, options *metav1.DeleteOptions) error {
	return p.client.Delete(name, options)
}

//func (p *notifiers) List(opts metav1.ListOptions) (runtime.Object, error) {
func (p *notifiers) List(opts metav1.ListOptions) (*NotifierList, error) {
	req := p.restClient.Get().
		Namespace(p.ns).
		Resource("notifiers").
		// VersionedParams(&options, v1.ParameterCodec)
		FieldsSelectorParam(nil)

	b, err := req.DoRaw()
	if err != nil {
		return nil, err
	}
	var prom NotifierList
	return &prom, json.Unmarshal(b, &prom)
}

func (p *notifiers) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	r, err := p.restClient.Get().
		Prefix("watch").
		Namespace(p.ns).
		Resource("notifiers").
		// VersionedParams(&options, v1.ParameterCodec).
		FieldsSelectorParam(nil).
		Stream()
	if err != nil {
		return nil, err
	}
	return watch.NewStreamWatcher(&notifierDecoder{
		dec:   json.NewDecoder(r),
		close: r.Close,
	}), nil
}

func (p *notifiers) DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error {
	return p.client.DeleteCollection(dopts, lopts)
}

func NotifierFromUnstructured(r *unstructured.Unstructured) (*Notifier, error) {
	b, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	var p Notifier
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	p.TypeMeta.Kind = NotifiersKind
	p.TypeMeta.APIVersion = Group + "/" + Version
	return &p, nil
}

func UnstructuredFromNotifier(p *Notifier) (*unstructured.Unstructured, error) {
	p.TypeMeta.Kind = NotifiersKind
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

type notifierDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *notifierDecoder) Close() {
	d.close()
}

func (d *notifierDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e struct {
		Type   watch.EventType
		Object Notifier
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}
