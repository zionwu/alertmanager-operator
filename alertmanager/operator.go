package alertmanager

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"

	alertconfig "github.com/zionwu/alertmanager-operator/alertmanager/config"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	ConfigFileName   = "config.yml"
	EnvLabelName     = "environment"
	AlertIDLabelName = "alert_id"
)

type Operator interface {
	AddReceiver(recipient *v1beta1.Recipient, Notifier *v1beta1.Notifier) error
	UpdateReceiver(recipient *v1beta1.RecipientList, Notifier *v1beta1.Notifier) error
	AddRoute(alert *v1beta1.Alert) error
}

type operator struct {
	client             *kubernetes.Clientset
	alertManagerUrl    string
	alertSecretName    string
	alertmanagerConfig string
}

func NewOperator(c *kubernetes.Clientset, alertManagerUrl string, alertSecretName string, alertmanagerConfig string) Operator {
	return &operator{
		client:             c,
		alertManagerUrl:    alertManagerUrl,
		alertSecretName:    alertSecretName,
		alertmanagerConfig: alertmanagerConfig,
	}
}

func (o *operator) AddReceiver(recipient *v1beta1.Recipient, notifier *v1beta1.Notifier) error {
	//1. get configuration from secret
	//TODO: should not hardcode the namespace
	sClient := o.client.CoreV1().Secrets("default")

	configSecret, err := sClient.Get(o.alertSecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	configBtyes := configSecret.Data[ConfigFileName]

	newConfigStr, err := o.addReceiver2Config(string(configBtyes), recipient, notifier)
	if err != nil {
		return err
	}

	configSecret.Data[ConfigFileName] = []byte(newConfigStr)
	_, err = sClient.Update(configSecret)
	if err != nil {
		return err
	}
	//reload alertmanager
	go o.reload()

	return nil

}

func (o *operator) UpdateReceiver(recipientList *v1beta1.RecipientList, notifier *v1beta1.Notifier) error {
	//1. get configuration from secret
	//TODO: should not hardcode the namespace
	sClient := o.client.CoreV1().Secrets("default")

	configSecret, err := sClient.Get(o.alertSecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	configBtyes := configSecret.Data[ConfigFileName]

	newConfigStr, err := o.updateReceiver2Config(string(configBtyes), recipientList, notifier)
	if err != nil {
		return err
	}

	configSecret.Data[ConfigFileName] = []byte(newConfigStr)
	_, err = sClient.Update(configSecret)
	if err != nil {
		return err
	}
	//reload alertmanager
	go o.reload()

	return nil

}

func (o *operator) AddRoute(alert *v1beta1.Alert) error {
	//1. get configuration from secret
	//TODO: should not hardcode the namespace
	sClient := o.client.CoreV1().Secrets("default")

	configSecret, err := sClient.Get(o.alertSecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	configBtyes := configSecret.Data[ConfigFileName]

	newConfigStr, err := o.addRoute2Config(string(configBtyes), alert)
	if err != nil {
		return err
	}

	configSecret.Data[ConfigFileName] = []byte(newConfigStr)
	_, err = sClient.Update(configSecret)
	if err != nil {
		return err
	}

	go o.reload()

	return nil

}

func (o *operator) addRoute2Config(configStr string, alert *v1beta1.Alert) (string, error) {
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	envRoutes := &config.Route.Routes
	if envRoutes == nil {
		*envRoutes = []*alertconfig.Route{}
	}
	env := alert.Labels["environment"]
	var envRoute *alertconfig.Route
	for _, r := range *envRoutes {
		if r.Match[EnvLabelName] == env {
			envRoute = r
			break
		}
	}

	if envRoute == nil {
		match := map[string]string{}
		match[EnvLabelName] = env
		envRoute = &alertconfig.Route{Match: match, Routes: []*alertconfig.Route{}, Continue: true}
		*envRoutes = append(*envRoutes, envRoute)
	}

	match := map[string]string{}
	match[AlertIDLabelName] = alert.Name
	route := &alertconfig.Route{
		Receiver: alert.Spec.RecipientID,
		Match:    match,
	}
	envRoute.Routes = append(envRoute.Routes, route)

	//update the secret
	d, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	logrus.Infof("Config file: %s", string(d))

	return string(d), nil
}

func (o *operator) addReceiver2Config(configStr string, recipient *v1beta1.Recipient, notifier *v1beta1.Notifier) (string, error) {
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	//2. add the receiver to the configuration
	receiver := &alertconfig.Receiver{Name: recipient.Name}
	switch recipient.Spec.Type {
	case "email":
		email := &alertconfig.EmailConfig{
			To:           recipient.Spec.EmailRecipient.Address,
			From:         notifier.Spec.EmailConfig.SMTPFrom,
			Smarthost:    notifier.Spec.EmailConfig.SMTPSmartHost,
			AuthUsername: notifier.Spec.EmailConfig.SMTPAuthUserName,
			AuthPassword: alertconfig.Secret(notifier.Spec.EmailConfig.SMTPAuthPassword),
			AuthSecret:   alertconfig.Secret(notifier.Spec.EmailConfig.SMTPAuthSecret),
			AuthIdentity: notifier.Spec.EmailConfig.SMTPAuthIdentity,
			RequireTLS:   &notifier.Spec.EmailConfig.SMTPRequireTLS,
		}
		receiver.EmailConfigs = append(receiver.EmailConfigs, email)

	case "slack":
		slack := &alertconfig.SlackConfig{
			Channel: recipient.Spec.SlackRecipient.Channel,
			APIURL:  alertconfig.Secret(notifier.Spec.SlackConfig.SlackApiUrl),
		}
		receiver.SlackConfigs = append(receiver.SlackConfigs, slack)
	case "pagerduty":
		pagerduty := &alertconfig.PagerdutyConfig{
			ServiceKey: alertconfig.Secret(recipient.Spec.PagerDutyRecipient.ServiceKey),
			URL:        notifier.Spec.PagerDutyConfig.PagerDutyUrl,
		}
		receiver.PagerdutyConfigs = append(receiver.PagerdutyConfigs, pagerduty)
	}

	config.Receivers = append(config.Receivers, receiver)

	//update the secret
	d, err := yaml.Marshal(config)

	logrus.Infof("Config file: %s", string(d))

	if err != nil {
		return "", err
	}

	return string(d), nil
}

func (o *operator) updateReceiver2Config(configStr string, recipientList *v1beta1.RecipientList, notifier *v1beta1.Notifier) (string, error) {
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	for _, receiver := range config.Receivers {
		for _, recipient := range recipientList.Items {
			if recipient.Name == receiver.Name {
				switch recipient.Spec.Type {
				case "email":
					email := &alertconfig.EmailConfig{
						To:           recipient.Spec.EmailRecipient.Address,
						From:         notifier.Spec.EmailConfig.SMTPFrom,
						Smarthost:    notifier.Spec.EmailConfig.SMTPSmartHost,
						AuthUsername: notifier.Spec.EmailConfig.SMTPAuthUserName,
						AuthPassword: alertconfig.Secret(notifier.Spec.EmailConfig.SMTPAuthPassword),
						AuthSecret:   alertconfig.Secret(notifier.Spec.EmailConfig.SMTPAuthSecret),
						AuthIdentity: notifier.Spec.EmailConfig.SMTPAuthIdentity,
						RequireTLS:   &notifier.Spec.EmailConfig.SMTPRequireTLS,
					}
					receiver.EmailConfigs[0] = email

				case "slack":
					slack := &alertconfig.SlackConfig{
						Channel: recipient.Spec.SlackRecipient.Channel,
						APIURL:  alertconfig.Secret(notifier.Spec.SlackConfig.SlackApiUrl),
					}
					receiver.SlackConfigs[0] = slack
				case "pagerduty":
					pagerduty := &alertconfig.PagerdutyConfig{
						ServiceKey: alertconfig.Secret(recipient.Spec.PagerDutyRecipient.ServiceKey),
						URL:        notifier.Spec.PagerDutyConfig.PagerDutyUrl,
					}
					receiver.PagerdutyConfigs[0] = pagerduty
					receiver.PagerdutyConfigs = append(receiver.PagerdutyConfigs, pagerduty)
				}
			}

		}

	}

	//update the secret
	d, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	logrus.Infof("Config file: %s", string(d))

	return string(d), nil
}

func (o *operator) reload() error {
	//TODO: what is the wait time
	time.Sleep(10000 * time.Millisecond)
	resp, err := http.Post(o.alertManagerUrl+"/-/reload", "text/html", nil)
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
