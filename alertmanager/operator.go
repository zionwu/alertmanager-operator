package alertmanager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/dispatch"
	alertconfig "github.com/zionwu/alertmanager-operator/alertmanager/config"
	"github.com/zionwu/alertmanager-operator/client"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	yaml "gopkg.in/yaml.v2"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	ConfigFileName   = "config.yml"
	NSLabelName      = "namespace"
	AlertIDLabelName = "alert_id"

	resyncPeriod = 5 * time.Minute
)

type Operator struct {
	kclient            *kubernetes.Clientset
	mclient            *client.Clientset
	alertManagerUrl    string
	alertSecretName    string
	alertmanagerConfig string

	crdclient    apiextensionsclient.Interface
	alertInf     cache.SharedIndexInformer
	notifiertInf cache.SharedIndexInformer
	recipentInf  cache.SharedIndexInformer
	//queue        workqueue.RateLimitingInterface
}

func NewOperator(config *rest.Config, alertManagerUrl string, alertSecretName string, alertmanagerConfig string) (*Operator, error) {
	// create the clientset
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating kubernetes client failed")
	}

	mclient, err := client.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating monitoring client failed")
	}

	crdclient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating apiextensions client failed")
	}

	o := &Operator{
		kclient:   kclient,
		mclient:   mclient,
		crdclient: crdclient,
		//queue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "alertmanager"),
		alertManagerUrl:    alertManagerUrl,
		alertSecretName:    alertSecretName,
		alertmanagerConfig: alertmanagerConfig,
	}

	o.alertInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  o.mclient.MonitoringV1().Alerts(api.NamespaceAll).List,
			WatchFunc: o.mclient.MonitoringV1().Alerts(api.NamespaceAll).Watch,
		},
		&v1beta1.Alert{}, resyncPeriod, cache.Indexers{},
	)

	o.recipentInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  o.mclient.MonitoringV1().Recipients(api.NamespaceAll).List,
			WatchFunc: o.mclient.MonitoringV1().Recipients(api.NamespaceAll).Watch,
		},
		&v1beta1.Recipient{}, resyncPeriod, cache.Indexers{},
	)

	o.notifiertInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  o.mclient.MonitoringV1().Notifiers(api.NamespaceAll).List,
			WatchFunc: o.mclient.MonitoringV1().Notifiers(api.NamespaceAll).Watch,
		},
		&v1beta1.Notifier{}, resyncPeriod, cache.Indexers{},
	)

	o.alertInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.handleAlertAdd,
		DeleteFunc: o.handleAlertDelete,
		UpdateFunc: o.handleAlertUpdate,
	})
	o.recipentInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.handleRecipientAdd,
		DeleteFunc: o.handleRecipientDelete,
		UpdateFunc: o.handleRecipientUpdate,
	})
	o.notifiertInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.handleNotifierAdd,
		DeleteFunc: o.handleNotifierDelete,
		UpdateFunc: o.handleNotifierUpdate,
	})

	return o, nil
}

// Run the controller.
func (c *Operator) Run(stopc <-chan struct{}) error {
	//defer c.queue.ShutDown()

	errChan := make(chan error)
	go func() {
		v, err := c.kclient.Discovery().ServerVersion()
		if err != nil {
			errChan <- errors.Wrap(err, "communicating with server failed")
			return
		}
		logrus.Infof("connection established, cluster-version: %v", v)
		/*
			TODO: create the CRD, ETA 10.31
			if err := c.createCRDs(); err != nil {
				errChan <- errors.Wrap(err, "creating CRDs failed")
				return
			}
		*/
		errChan <- nil
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
		logrus.Info("CRD API endpoints ready")
	case <-stopc:
		return nil
	}

	//go c.worker()

	go c.alertInf.Run(stopc)
	go c.recipentInf.Run(stopc)
	go c.notifiertInf.Run(stopc)

	<-stopc
	return nil
}

func (c *Operator) handleAlertAdd(obj interface{}) {
	alert := obj.(*v1beta1.Alert)
	logrus.Infof("Add for alert: %v", alert)

	if err := c.makeConfig(alert, c.addRoute2Config); err != nil {
		logrus.Errorf("Error whiling adding route: %v", err)
	}
}

func (c *Operator) handleAlertDelete(obj interface{}) {
	alert := obj.(*v1beta1.Alert)
	logrus.Infof("Delete for alert: %v", alert)

	if err := c.makeConfig(alert, c.deleteRoute2Config); err != nil {
		logrus.Errorf("Error whiling deleting route: %v", err)
	}
}

func (c *Operator) handleAlertUpdate(oldObj, curObj interface{}) {
	alert := curObj.(*v1beta1.Alert)
	oldAlert := oldObj.(*v1beta1.Alert)

	if alert.GetResourceVersion() == oldAlert.GetResourceVersion() {
		logrus.Infof("Same version: %v", alert.GetResourceVersion())
		return
	}

	logrus.Infof("Update for alert: %v", alert)

	if err := c.makeConfig(alert, c.updateRoute2Config); err != nil {
		logrus.Errorf("Error whiling updating route: %v", err)
	}
}

func (c *Operator) handleRecipientAdd(obj interface{}) {
	recipient := obj.(*v1beta1.Recipient)
	logrus.Infof("Add for recipient: %v", recipient)

	if err := c.makeConfig(recipient, c.addReceiver2Config); err != nil {
		logrus.Errorf("Error whiling adding receiver: %v", err)
	}
}

func (c *Operator) handleRecipientDelete(obj interface{}) {
	recipient := obj.(*v1beta1.Recipient)
	logrus.Infof("Delete for recipient: %v", recipient)

	if err := c.makeConfig(recipient, c.deleteReceiver2Config); err != nil {
		logrus.Errorf("Error whiling deleting receiver: %v", err)
	}
}

func (c *Operator) handleRecipientUpdate(oldObj, curObj interface{}) {
	recipient := curObj.(*v1beta1.Recipient)
	oldRecipient := oldObj.(*v1beta1.Recipient)

	if recipient.GetResourceVersion() == oldRecipient.GetResourceVersion() {
		logrus.Infof("Same version: %v", recipient.GetResourceVersion())
		return
	}
	logrus.Infof("Update for recipient: %v", recipient)

	if err := c.makeConfig(recipient, c.updateReceiver2Config); err != nil {
		logrus.Errorf("Error whiling updating receiver: %v", err)
	}
}

func (c *Operator) handleNotifierAdd(obj interface{}) {

	notifier := obj.(*v1beta1.Notifier)
	logrus.Infof("Add for notifier: %v", notifier)

	if err := c.makeConfig(notifier, c.updateNotifier2Config); err != nil {
		logrus.Errorf("Error whiling adding notifier: %v", err)
	}
}

func (c *Operator) handleNotifierDelete(obj interface{}) {
	/*
		recipient := curObj.(*v1beta1.Recipient)
		if err := c.updateReceiver(recipient, c.updateReceiver2Config); err != nil {
			logrus.Errorf("Error whiling updating notifier: %v", err)
		}
	*/
}

func (c *Operator) handleNotifierUpdate(oldObj, curObj interface{}) {
	notifier := curObj.(*v1beta1.Notifier)
	oldNotifier := oldObj.(*v1beta1.Notifier)

	if notifier.GetResourceVersion() == oldNotifier.GetResourceVersion() {
		logrus.Infof("Same version: %v", notifier.GetResourceVersion())
		return
	}

	logrus.Infof("Update for notifier: %v", notifier)

	if err := c.makeConfig(notifier, c.updateNotifier2Config); err != nil {
		logrus.Errorf("Error whiling updating notifier: %v", err)
	}

}

func (c *Operator) makeConfig(obj interface{}, f func(string, interface{}) (string, error)) error {
	//1. get configuration from secret
	//TODO: should not hardcode the namespace
	sClient := c.kclient.CoreV1().Secrets("default")

	configSecret, err := sClient.Get(c.alertSecretName, metav1.GetOptions{})
	if err != nil {
		logrus.Error("Error while getting secret: %v", err)
		return err
	}

	configBtyes := configSecret.Data[ConfigFileName]

	newConfigStr, err := f(string(configBtyes), obj)
	if err != nil {
		logrus.Error("Error while adding config: %v", err)
		return err
	}

	configSecret.Data[ConfigFileName] = []byte(newConfigStr)
	_, err = sClient.Update(configSecret)
	if err != nil {
		logrus.Error("Error while updating secret: %v", err)
		return err
	}
	//reload alertmanager
	go c.reload()

	return nil
}

func (c *Operator) addRoute2Config(configStr string, a interface{}) (string, error) {
	logrus.Infof("before adding route: %s", configStr)

	alert := a.(*v1beta1.Alert)

	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	envRoutes := &config.Route.Routes
	if envRoutes == nil {
		*envRoutes = []*alertconfig.Route{}
	}

	var envRoute *alertconfig.Route
	for _, r := range *envRoutes {
		if r.Match[NSLabelName] == alert.Namespace {
			envRoute = r
			break
		}
	}

	if envRoute == nil {
		match := map[string]string{}
		match[NSLabelName] = alert.Namespace
		envRoute = &alertconfig.Route{Match: match, Routes: []*alertconfig.Route{}}
		*envRoutes = append(*envRoutes, envRoute)
	}

	for _, route := range envRoute.Routes {
		if route.Match[AlertIDLabelName] == alert.Name {
			return configStr, nil
		}
	}

	match := map[string]string{}
	match[AlertIDLabelName] = alert.Name
	route := &alertconfig.Route{
		Receiver: alert.RecipientID,
		Match:    match,
	}
	envRoute.Routes = append(envRoute.Routes, route)

	//update the secret
	d, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	logrus.Infof("after adding route: %s", string(d))

	return string(d), nil
}

func (c *Operator) updateRoute2Config(configStr string, a interface{}) (string, error) {
	logrus.Infof("before updating route: %s", configStr)

	alert := a.(*v1beta1.Alert)

	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	envRoutes := &config.Route.Routes

	var envRoute *alertconfig.Route
	for _, r := range *envRoutes {
		if r.Match[NSLabelName] == alert.Namespace {
			envRoute = r
			break
		}
	}

	for _, route := range envRoute.Routes {
		if route.Match[AlertIDLabelName] == alert.Name {
			route.Receiver = alert.RecipientID
			break
		}
	}

	//update the secret
	d, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	logrus.Infof("after updating route: %s", string(d))

	return string(d), nil
}

func (c *Operator) deleteRoute2Config(configStr string, a interface{}) (string, error) {
	logrus.Infof("before deleting route: %s", configStr)
	alert := a.(*v1beta1.Alert)
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	envRoutes := &config.Route.Routes

	var envRoute *alertconfig.Route
	for _, r := range *envRoutes {
		if r.Match[NSLabelName] == alert.Namespace {
			envRoute = r
			break
		}
	}

	routes := envRoute.Routes
	for i, route := range routes {
		if route.Match[AlertIDLabelName] == alert.Name {
			routes = append(routes[:i], routes[i+1:]...)
			break
		}
	}
	envRoute.Routes = routes

	//update the secret
	d, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	logrus.Infof("after deleting route: %s", string(d))

	return string(d), nil
}

func (c *Operator) addReceiver2Config(configStr string, r interface{}) (string, error) {
	logrus.Infof("before adding receiver: %s", configStr)
	recipient := r.(*v1beta1.Recipient)
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	for _, item := range config.Receivers {
		if item.Name == recipient.Name {
			return configStr, nil
		}
	}

	//2. add the receiver to the configuration
	receiver := &alertconfig.Receiver{Name: recipient.Name}
	if recipient.EmailRecipient.Address != "" {
		email := &alertconfig.EmailConfig{
			To: recipient.EmailRecipient.Address,
		}
		receiver.EmailConfigs = append(receiver.EmailConfigs, email)
	} else if recipient.SlackRecipient.Channel != "" {
		slack := &alertconfig.SlackConfig{
			//TODO: set a better text content
			Channel: recipient.SlackRecipient.Channel,
			Text:    "pod {{ (index .Alerts 0).Labels.object_id}} is unhealthy",
			Title:   "Alert From Rancher",
		}
		receiver.SlackConfigs = append(receiver.SlackConfigs, slack)
	} else if recipient.PagerDutyRecipient.ServiceKey != "" {
		pagerduty := &alertconfig.PagerdutyConfig{
			ServiceKey: alertconfig.Secret(recipient.PagerDutyRecipient.ServiceKey),
		}
		receiver.PagerdutyConfigs = append(receiver.PagerdutyConfigs, pagerduty)
	}

	config.Receivers = append(config.Receivers, receiver)

	//update the secret
	d, err := yaml.Marshal(config)

	logrus.Infof("after adding receiver: %s", string(d))

	if err != nil {
		return "", err
	}

	return string(d), nil
}

func (c *Operator) updateReceiver2Config(configStr string, r interface{}) (string, error) {
	logrus.Infof("before deleting receiver: %s", configStr)
	recipient := r.(*v1beta1.Recipient)
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	for _, item := range config.Receivers {
		if item.Name == recipient.Name {
			if recipient.EmailRecipient.Address != "" {
				email := &alertconfig.EmailConfig{
					To: recipient.EmailRecipient.Address,
				}
				item.EmailConfigs[0] = email
			} else if recipient.SlackRecipient.Channel != "" {
				slack := &alertconfig.SlackConfig{
					Channel: recipient.SlackRecipient.Channel,
					Text:    "pod {{ (index .Alerts 0).Labels.object_id}} is unhealthy",
					Title:   "Alert From Rancher",
				}
				item.SlackConfigs[0] = slack
			} else if recipient.PagerDutyRecipient.ServiceKey != "" {
				pagerduty := &alertconfig.PagerdutyConfig{
					ServiceKey: alertconfig.Secret(recipient.PagerDutyRecipient.ServiceKey),
				}
				item.PagerdutyConfigs[0] = pagerduty
			}
		}
	}
	//update the secret
	d, err := yaml.Marshal(config)
	logrus.Infof("after deleting receiver: %s", string(d))
	if err != nil {
		return "", err
	}

	return string(d), nil
}

func (c *Operator) deleteReceiver2Config(configStr string, r interface{}) (string, error) {
	logrus.Infof("before deleting receiver: %s", configStr)
	recipient := r.(*v1beta1.Recipient)
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	receivers := config.Receivers
	for i, item := range receivers {
		if item.Name == recipient.Name {
			receivers = append(receivers[:i], receivers[i+1:]...)
			break
		}
	}
	config.Receivers = receivers
	//update the secret
	d, err := yaml.Marshal(config)
	logrus.Infof("after deleting receiver: %s", string(d))
	if err != nil {
		return "", err
	}

	return string(d), nil
}

func (c *Operator) updateNotifier2Config(configStr string, n interface{}) (string, error) {
	logrus.Infof("before updating notifier: %s", configStr)
	notifier := n.(*v1beta1.Notifier)
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}
	if notifier.PagerDutyConfig != nil {
		config.Global.PagerdutyURL = notifier.PagerDutyConfig.PagerDutyUrl
	}

	if notifier.SlackConfig != nil {
		config.Global.SlackAPIURL = alertconfig.Secret(notifier.SlackConfig.SlackApiUrl)

	}

	if notifier.EmailConfig != nil {
		config.Global.SMTPAuthIdentity = notifier.EmailConfig.SMTPAuthIdentity
		config.Global.SMTPAuthPassword = alertconfig.Secret(notifier.EmailConfig.SMTPAuthPassword)
		config.Global.SMTPAuthSecret = alertconfig.Secret(notifier.EmailConfig.SMTPAuthSecret)
		config.Global.SMTPAuthUsername = notifier.EmailConfig.SMTPAuthUserName
		config.Global.SMTPFrom = notifier.EmailConfig.SMTPFrom
		config.Global.SMTPSmarthost = notifier.EmailConfig.SMTPSmartHost
		config.Global.SMTPRequireTLS = notifier.EmailConfig.SMTPRequireTLS
	}

	//update the secret
	d, err := yaml.Marshal(config)
	logrus.Infof("after updating notifier: %s", string(d))
	if err != nil {
		return "", err
	}

	return string(d), nil
}

func (c *Operator) reload() error {
	//TODO: what is the wait time
	time.Sleep(10 * time.Second)
	resp, err := http.Post(c.alertManagerUrl+"/-/reload", "text/html", nil)
	logrus.Infof("Reload alert manager configuration")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

//TODO: decide which package should this function be
func (c *Operator) getActiveAlertListFromAlertManager(filter string) ([]*dispatch.APIAlert, error) {

	res := struct {
		Data   []*dispatch.APIAlert `json:"data"`
		Status string               `json:"status"`
	}{}

	req, err := http.NewRequest(http.MethodGet, c.alertManagerUrl+"/api/v1/alerts", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("filter", fmt.Sprintf("{%s}", filter))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	requestBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(requestBytes, &res); err != nil {
		return nil, err
	}

	return res.Data, nil
}
