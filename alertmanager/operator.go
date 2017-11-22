package alertmanager

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"

	alertconfig "github.com/zionwu/alertmanager-operator/alertmanager/config"
	alertapi "github.com/zionwu/alertmanager-operator/api"

	"github.com/zionwu/alertmanager-operator/client"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	"github.com/zionwu/alertmanager-operator/util"
	"github.com/zionwu/alertmanager-operator/watch"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	ConfigFileName   = "config.yml"
	NSLabelName      = "namespace"
	AlertIDLabelName = "alert_id"

	resyncPeriod = 0
)

type Operator struct {
	kclient kubernetes.Interface
	mclient client.Interface
	cfg     *alertapi.Config

	crdclient    apiextensionsclient.Interface
	alertInf     cache.SharedIndexInformer
	notifiertInf cache.SharedIndexInformer
	recipentInf  cache.SharedIndexInformer

	synchronizer Synchronizer
	queue        workqueue.RateLimitingInterface
	watchers     map[string]watch.Watcher
}

func NewOperator(config *rest.Config, cfg *alertapi.Config) (*Operator, error) {
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
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "alertmanager"),
		cfg:       cfg,
		watchers:  map[string]watch.Watcher{},
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
			ListFunc:  o.mclient.MonitoringV1().Notifiers().List,
			WatchFunc: o.mclient.MonitoringV1().Notifiers().Watch,
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

	o.synchronizer = NewSynchronizer(cfg, mclient)

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

		if err := c.createCRDs(); err != nil {
			errChan <- errors.Wrap(err, "creating CRDs failed")
			return
		}

		if err := c.createNotifier(); err != nil {
			errChan <- errors.Wrap(err, "creating Notifier failed")
			return
		}

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

	go c.worker()
	go c.alertInf.Run(stopc)
	go c.recipentInf.Run(stopc)
	go c.notifiertInf.Run(stopc)
	go c.synchronizer.Run(stopc)

	<-stopc
	return nil
}

func (c *Operator) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Operator) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(key)
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(errors.Wrap(err, fmt.Sprintf("Sync %q failed", key)))
	c.queue.AddRateLimited(key)

	return true
}

func (c *Operator) sync(key interface{}) error {
	notifier, err := c.mclient.MonitoringV1().Notifiers().Get("rancher-notifier", metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting notifier: %v", err)
		return err
	}
	nSecret, err := c.kclient.Core().Secrets("default").Get("rancher-notifier", metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting notifier secret: %v", err)
		return err
	}

	al, err := c.mclient.MonitoringV1().Alerts(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing alert: %v", err)
		return err
	}
	alertList := al.(*v1beta1.AlertList)

	rl, err := c.mclient.MonitoringV1().Recipients(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing recipient: %v", err)
		return err
	}
	recipientList := rl.(*v1beta1.RecipientList)

	config := getDefaultConfig()

	config.Global.PagerdutyURL = "https://events.pagerduty.com/generic/2010-04-15/create_event.json"

	if notifier.SlackConfig != nil && notifier.SlackConfig.SlackApiUrl != "" {
		slackApiUrl := string(nSecret.Data["slackApiUrl"])
		config.Global.SlackAPIURL = alertconfig.Secret(slackApiUrl)

	}

	if notifier.EmailConfig != nil {
		//config.Global.SMTPAuthIdentity = notifier.EmailConfig.SMTPAuthIdentity
		smtpAuthPassword := string(nSecret.Data["smtpAuthPassword"])
		config.Global.SMTPAuthPassword = alertconfig.Secret(smtpAuthPassword)
		//config.Global.SMTPAuthSecret = alertconfig.Secret(notifier.EmailConfig.SMTPAuthSecret)
		config.Global.SMTPAuthUsername = notifier.EmailConfig.SMTPAuthUserName
		config.Global.SMTPFrom = notifier.EmailConfig.SMTPAuthUserName
		config.Global.SMTPSmarthost = notifier.EmailConfig.SMTPSmartHost
		config.Global.SMTPRequireTLS = false
	}

	for _, recipient := range recipientList.Items {
		c.addReceiver2Config(config, recipient)
	}

	for _, alert := range alertList.Items {
		c.addRoute2Config(config, alert)
	}

	sClient := c.kclient.CoreV1().Secrets("default")

	configSecret, err := sClient.Get(c.cfg.SecretName, metav1.GetOptions{})
	if err != nil {
		logrus.Error("Error while getting secret: %v", err)
		return err
	}

	d, err := yaml.Marshal(config)
	logrus.Debugf("after updating notifier: %s", string(d))
	if err != nil {
		return err
	}

	configSecret.Data[ConfigFileName] = d
	_, err = sClient.Update(configSecret)
	if err != nil {
		logrus.Error("Error while updating secret: %v", err)
		return err
	}
	//reload alertmanager
	go util.ReloadConfiguration(c.cfg.ManagerUrl)

	return nil

}

func getDefaultConfig() *alertconfig.Config {
	config := alertconfig.Config{}

	resolveTimeout, _ := model.ParseDuration("5m")
	config.Global = &alertconfig.GlobalConfig{
		SlackAPIURL:    "slack_api_url",
		ResolveTimeout: resolveTimeout,
		SMTPRequireTLS: false,
	}

	slackConfigs := []*alertconfig.SlackConfig{}
	initSlackConfig := &alertconfig.SlackConfig{
		Channel: "#alert",
	}
	slackConfigs = append(slackConfigs, initSlackConfig)

	receivers := []*alertconfig.Receiver{}
	initReceiver := &alertconfig.Receiver{
		Name:         "rancherlabs",
		SlackConfigs: slackConfigs,
	}
	receivers = append(receivers, initReceiver)

	config.Receivers = receivers

	groupWait, _ := model.ParseDuration("1m")
	groupInterval, _ := model.ParseDuration("0m")
	repeatInterval, _ := model.ParseDuration("1h")

	config.Route = &alertconfig.Route{
		Receiver:       "rancherlabs",
		GroupWait:      &groupWait,
		GroupInterval:  &groupInterval,
		RepeatInterval: &repeatInterval,
	}

	return &config
}

func convertToAlert(o interface{}) (*v1beta1.Alert, error) {
	alert, ok := o.(*v1beta1.Alert)
	if !ok {
		deletedState, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("Received unexpected object: %v", o)
		}
		alert, ok = deletedState.Obj.(*v1beta1.Alert)
		if !ok {
			return nil, fmt.Errorf("DeletedFinalStateUnknown contained non-Pod object: %v", deletedState.Obj)
		}
	}

	return alert, nil
}

func (c *Operator) handleAlertAdd(obj interface{}) {
	alert, err := convertToAlert(obj)
	if err != nil {
		logrus.Error("converting to Alert object failed")
		return
	}

	logrus.Debugf("Add for alert: %v", alert)

	//TODO: should we move watcher into sync?
	watcher := watch.NewWatcher(alert, c.kclient, c.cfg)
	c.watchers[alert.Name] = watcher
	go watcher.Watch()

	c.queue.Add(alert)
}

func (c *Operator) handleAlertDelete(obj interface{}) {
	alert, err := convertToAlert(obj)
	if err != nil {
		logrus.Error("converting to Alert object failed")
		return
	}

	logrus.Debugf("Delete for alert: %v", alert)

	c.watchers[alert.Name].Stop()
	delete(c.watchers, alert.Name)

	c.queue.Add(alert)

}

func (c *Operator) handleAlertUpdate(oldObj, curObj interface{}) {
	alert, err := convertToAlert(curObj)
	if err != nil {
		logrus.Error("converting to Alert object failed")
		return
	}

	oldAlert, err := convertToAlert(oldObj)
	if err != nil {
		logrus.Error("converting to Alert object failed")
		return
	}

	//TODO: do we need this block
	if alert.GetResourceVersion() == oldAlert.GetResourceVersion() {
		logrus.Infof("Same version: %v", alert.GetResourceVersion())
		return
	}

	c.watchers[alert.Name].UpdateAlert(alert)

	logrus.Infof("Update for alert: %v", alert)

	c.queue.Add(alert)
}

func convertRecipient(o interface{}) (*v1beta1.Recipient, error) {
	r, ok := o.(*v1beta1.Recipient)
	if !ok {
		deletedState, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("Received unexpected object: %v", o)
		}
		r, ok = deletedState.Obj.(*v1beta1.Recipient)
		if !ok {
			return nil, fmt.Errorf("DeletedFinalStateUnknown contained non-Pod object: %v", deletedState.Obj)
		}
	}

	return r, nil
}

func (c *Operator) handleRecipientAdd(obj interface{}) {
	recipient, err := convertRecipient(obj)
	if err != nil {
		logrus.Error("converting to Recipient object failed")
		return
	}
	logrus.Debugf("Add for recipient: %v", recipient)

	c.queue.Add(recipient)
}

func (c *Operator) handleRecipientDelete(obj interface{}) {
	recipient, err := convertRecipient(obj)
	if err != nil {
		logrus.Error("converting to Recipient object failed")
		return
	}
	logrus.Debugf("Delete for recipient: %v", recipient)

	c.queue.Add(recipient)
}

func (c *Operator) handleRecipientUpdate(oldObj, curObj interface{}) {
	recipient, err := convertRecipient(curObj)
	if err != nil {
		logrus.Error("converting to Recipient object failed")
		return
	}
	oldRecipient, err := convertRecipient(oldObj)
	if err != nil {
		logrus.Error("converting to Recipient object failed")
		return
	}
	logrus.Debugf("Update for recipient: %v", recipient)

	if recipient.GetResourceVersion() == oldRecipient.GetResourceVersion() {
		logrus.Infof("Same version: %v", recipient.GetResourceVersion())
		return
	}

	c.queue.Add(recipient)
}

func convertNotifier(o interface{}) (*v1beta1.Notifier, error) {
	r, ok := o.(*v1beta1.Notifier)
	if !ok {
		deletedState, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("Received unexpected object: %v", o)
		}
		r, ok = deletedState.Obj.(*v1beta1.Notifier)
		if !ok {
			return nil, fmt.Errorf("DeletedFinalStateUnknown contained non-Pod object: %v", deletedState.Obj)
		}
	}

	return r, nil
}

func (c *Operator) handleNotifierAdd(obj interface{}) {
	notifier, err := convertNotifier(obj)
	if err != nil {
		logrus.Error("converting to Notifier object failed")
		return
	}

	logrus.Debugf("Add for notifier: %v", notifier)
	c.queue.Add(notifier)
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
	notifier, err := convertNotifier(curObj)
	if err != nil {
		logrus.Error("converting to Notifier object failed")
		return
	}

	oldNotifier, err := convertNotifier(oldObj)
	if err != nil {
		logrus.Error("converting to Notifier object failed")
		return
	}

	if notifier.GetResourceVersion() == oldNotifier.GetResourceVersion() {
		logrus.Infof("Same version: %v", notifier.GetResourceVersion())
		return
	}

	logrus.Debugf("Update for notifier: %v", notifier)
	c.queue.Add(notifier)
}

func (c *Operator) addRoute2Config(config *alertconfig.Config, alert *v1beta1.Alert) error {

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
		envRoute = &alertconfig.Route{
			Match:  match,
			Routes: []*alertconfig.Route{},
		}
		*envRoutes = append(*envRoutes, envRoute)
	}

	match := map[string]string{}
	match[AlertIDLabelName] = alert.Name
	route := &alertconfig.Route{
		Receiver: alert.RecipientID,
		Match:    match,
	}
	if alert.AdvancedOptions != nil {
		gw, err := model.ParseDuration(alert.AdvancedOptions.InitialWait)
		if err == nil {
			route.GroupWait = &gw
		}
		ri, err := model.ParseDuration(alert.AdvancedOptions.RepeatInterval)
		if err == nil {
			route.RepeatInterval = &ri
		}

	}

	envRoute.Routes = append(envRoute.Routes, route)

	return nil
}

func (c *Operator) addReceiver2Config(config *alertconfig.Config, recipient *v1beta1.Recipient) error {

	receiver := &alertconfig.Receiver{Name: recipient.Name}
	switch recipient.RecipientType {
	case "webhook":
		webhook := &alertconfig.WebhookConfig{
			URL: recipient.WebhookRecipient.URL,
		}
		receiver.WebhookConfigs = append(receiver.WebhookConfigs, webhook)

	case "email":
		header := map[string]string{}
		header["Subject"] = "Alert from Rancher: {{ (index .Alerts 0).Labels.description}}"
		email := &alertconfig.EmailConfig{
			To:      recipient.EmailRecipient.Address,
			Headers: header,
			//HTML:    "Resource Type:  {{ (index .Alerts 0).Labels.target_type}}\nResource Name:  {{ (index .Alerts 0).Labels.target_id}}\nNamespace:  {{ (index .Alerts 0).Labels.namespace}}\n",
		}
		receiver.EmailConfigs = append(receiver.EmailConfigs, email)
	case "slack":
		slack := &alertconfig.SlackConfig{
			//TODO: set a better text content
			Channel: recipient.SlackRecipient.Channel,
			Text:    "Resource Type:  {{ (index .Alerts 0).Labels.target_type}}\nResource Name:  {{ (index .Alerts 0).Labels.target_id}}\nNamespace:  {{ (index .Alerts 0).Labels.namespace}}\n",
			Title:   "{{ (index .Alerts 0).Labels.description}}",
			Pretext: "Alert From Rancher",
			Color:   `{{ if eq (index .Alerts 0).Labels.severity "critical" }}danger{{ else if eq (index .Alerts 0).Labels.severity "warning" }}warning{{ else }}good{{ end }}`,
		}
		receiver.SlackConfigs = append(receiver.SlackConfigs, slack)
	case "pagerduty":
		secret, err := c.kclient.Core().Secrets(recipient.Namespace).Get(recipient.Name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Error while getting pagerduty secret: %v", err)
			return err
		}
		serviceKey := string(secret.Data["serviceKey"])

		pagerduty := &alertconfig.PagerdutyConfig{
			ServiceKey:  alertconfig.Secret(serviceKey),
			Description: "{{ (index .Alerts 0).Labels.description}}",
		}
		receiver.PagerdutyConfigs = append(receiver.PagerdutyConfigs, pagerduty)
	}

	config.Receivers = append(config.Receivers, receiver)

	return nil
}

func (c *Operator) createCRDs() error {
	crds := []*extensionsobj.CustomResourceDefinition{
		util.NewAlertCustomResourceDefinition(),
		util.NewNotifierCustomResourceDefinition(),
		util.NewRecipientCustomResourceDefinition(),
	}

	crdClient := c.crdclient.ApiextensionsV1beta1().CustomResourceDefinitions()

	for _, crd := range crds {
		if _, err := crdClient.Create(crd); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "Creating CRD: %s", crd.Spec.Names.Kind)
		}
		logrus.Infof("msg", "CRD created", "crd", crd.Spec.Names.Kind)
	}

	// We have to wait for the CRDs to be ready. Otherwise the initial watch may fail.
	err := util.WaitForCRDReady(c.mclient.MonitoringV1().Alerts(api.NamespaceAll).List)
	if err != nil {
		return err
	}
	err = util.WaitForCRDReady(c.mclient.MonitoringV1().Notifiers().List)
	if err != nil {
		return err
	}
	return util.WaitForCRDReady(c.mclient.MonitoringV1().Recipients(api.NamespaceAll).List)
}

func (c *Operator) createNotifier() error {

	nclient := c.mclient.MonitoringV1().Notifiers()
	notifier := &v1beta1.Notifier{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rancher-notifier",
		},
		EmailConfig: &v1beta1.EmailConfigSpec{},
		SlackConfig: &v1beta1.SlackConfigSpec{},

		//PagerDutyConfig: &v1beta1.PagerDutyConfigSpec{
		//	PagerDutyUrl: "https://events.pagerduty.com/generic/2010-04-15/create_event.json",
		//},
	}
	_, err := nclient.Create(notifier)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Creating notifier")
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rancher-notifier",
		},
		Data: map[string][]byte{},
	}
	_, err = c.kclient.Core().Secrets("default").Create(secret)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Creating notifier secrets")
	}

	return nil

}
