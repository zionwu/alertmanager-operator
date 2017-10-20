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
	RecipientsKind        = "Recipient"
	RecipientName         = "recipients"
	RecipientSingularName = "recipient"
)

type RecipientsGetter interface {
	Recipients(namespace string) RecipientInterface
}

//only used to check if the recipients struct implements RecipientInterface
var _ RecipientInterface = &recipients{}

type RecipientInterface interface {
	Create(*Recipient) (*Recipient, error)
	Get(name string, opts metav1.GetOptions) (*Recipient, error)
	Update(*Recipient) (*Recipient, error)
	Delete(name string, options *metav1.DeleteOptions) error
	//List(opts metav1.ListOptions) (runtime.Object, error)
	List(opts metav1.ListOptions) (*RecipientList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error
}

type recipients struct {
	restClient rest.Interface
	client     *dynamic.ResourceClient
	ns         string
}

func newRecipients(r rest.Interface, c *dynamic.Client, namespace string) *recipients {
	return &recipients{
		r,
		c.Resource(
			&metav1.APIResource{
				Kind:       RecipientsKind,
				Name:       RecipientName,
				Namespaced: true,
			},
			namespace,
		),
		namespace,
	}
}

func (p *recipients) Create(o *Recipient) (*Recipient, error) {
	up, err := UnstructuredFromRecipient(o)
	if err != nil {
		return nil, err
	}

	up, err = p.client.Create(up)
	if err != nil {
		return nil, err
	}

	return RecipientFromUnstructured(up)
}

func (p *recipients) Get(name string, opts metav1.GetOptions) (*Recipient, error) {
	obj, err := p.client.Get(name, opts)
	if err != nil {
		return nil, err
	}
	return RecipientFromUnstructured(obj)
}

func (p *recipients) Update(o *Recipient) (*Recipient, error) {
	up, err := UnstructuredFromRecipient(o)
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

	return RecipientFromUnstructured(up)
}

func (p *recipients) Delete(name string, options *metav1.DeleteOptions) error {
	return p.client.Delete(name, options)
}

//func (p *recipients) List(opts metav1.ListOptions) (runtime.Object, error) {
func (p *recipients) List(opts metav1.ListOptions) (*RecipientList, error) {
	labelSelector, err := labels.Parse(opts.LabelSelector)
	if err != nil {
		return nil, err
	}

	req := p.restClient.Get().
		Namespace(p.ns).
		Resource(RecipientName).
		// VersionedParams(&options, v1.ParameterCodec)
		FieldsSelectorParam(nil).LabelsSelectorParam(labelSelector)

	b, err := req.DoRaw()
	if err != nil {
		return nil, err
	}
	var prom RecipientList
	return &prom, json.Unmarshal(b, &prom)
}

func (p *recipients) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	r, err := p.restClient.Get().
		Prefix("watch").
		Namespace(p.ns).
		Resource(RecipientName).
		// VersionedParams(&options, v1.ParameterCodec).
		FieldsSelectorParam(nil).
		Stream()
	if err != nil {
		return nil, err
	}
	return watch.NewStreamWatcher(&recipientDecoder{
		dec:   json.NewDecoder(r),
		close: r.Close,
	}), nil
}

func (p *recipients) DeleteCollection(dopts *metav1.DeleteOptions, lopts metav1.ListOptions) error {
	return p.client.DeleteCollection(dopts, lopts)
}

func RecipientFromUnstructured(r *unstructured.Unstructured) (*Recipient, error) {
	b, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	var p Recipient
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	p.TypeMeta.Kind = RecipientsKind
	p.TypeMeta.APIVersion = Group + "/" + Version
	return &p, nil
}

func UnstructuredFromRecipient(p *Recipient) (*unstructured.Unstructured, error) {
	p.TypeMeta.Kind = RecipientsKind
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

type recipientDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *recipientDecoder) Close() {
	d.close()
}

func (d *recipientDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e struct {
		Type   watch.EventType
		Object Recipient
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}
