package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
)

const (
	Group = "monitoring.rancher.io"
)

var Version = "v1beta1"

type MonitoringV1Interface interface {
	RESTClient() rest.Interface
	NotifiersGetter
	RecipientsGetter
	AlertsGetter
}

type MonitoringV1Client struct {
	restClient    rest.Interface
	dynamicClient *dynamic.Client
}

func (c *MonitoringV1Client) Notifiers(namespace string) NotifierInterface {
	return newNotifiers(c.restClient, c.dynamicClient, namespace)
}

func (c *MonitoringV1Client) Recipients(namespace string) RecipientInterface {
	return newRecipients(c.restClient, c.dynamicClient, namespace)
}

func (c *MonitoringV1Client) Alerts(namespace string) AlertInterface {
	return newAlerts(c.restClient, c.dynamicClient, namespace)
}

func (c *MonitoringV1Client) RESTClient() rest.Interface {
	return c.restClient
}

func NewForConfig(c *rest.Config) (*MonitoringV1Client, error) {
	config := *c
	SetConfigDefaults(&config)
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewClient(&config)
	if err != nil {
		return nil, err
	}

	return &MonitoringV1Client{client, dynamicClient}, nil
}

func SetConfigDefaults(config *rest.Config) {
	config.GroupVersion = &schema.GroupVersion{
		Group:   Group,
		Version: Version,
	}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}
	return
}
