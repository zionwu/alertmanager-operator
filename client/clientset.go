package client

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"

	"github.com/zionwu/alertmanager-operator/client/v1beta1"
)

var _ Interface = &Clientset{}

type Interface interface {
	MonitoringV1() v1beta1.MonitoringV1Interface
}

type Clientset struct {
	*v1beta1.MonitoringV1Client
}

func (c *Clientset) MonitoringV1() v1beta1.MonitoringV1Interface {
	if c == nil {
		return nil
	}
	return c.MonitoringV1Client
}

func NewForConfig(c *rest.Config) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}
	var cs Clientset
	var err error

	cs.MonitoringV1Client, err = v1beta1.NewForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	return &cs, nil
}
