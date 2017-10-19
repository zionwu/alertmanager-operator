package alertmanager

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	alertconfig "github.com/prometheus/alertmanager/config"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	"github.com/zionwu/alertmanager-operator/util"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Operator interface {
	AddReceiver(recipient *v1beta1.Recipient, Notifier *v1beta1.Notifier, c *kubernetes.Clientset) error
	AddRoute(alert *v1beta1.Alert, c *kubernetes.Clientset) error
}

type operator struct {
}

func NewOperator() Operator {
	return &operator{}
}

func (o *operator) AddReceiver(recipient *v1beta1.Recipient, notifier *v1beta1.Notifier, c *kubernetes.Clientset) error {
	//1. get configuration from secret
	//TODO: should not hardcode the namespace
	sClient := c.CoreV1().Secrets("default")

	//TODO: should not hardcode the secret name
	configSecret, err := sClient.Get("alertmanager-config2", metav1.GetOptions{})
	if err != nil {
		return err
	}
	//TODO: should not hardcode the file name
	configBtyes := configSecret.Data["config.yml"]
	configStr := util.DecodeBase64(string(configBtyes))

	newConfigStr, err := o.addReceiver2Config(configStr, recipient, notifier)
	if err != nil {
		return err
	}

	configSecret.Data["config.yml"] = []byte(newConfigStr)
	_, err = sClient.Update(configSecret)
	if err != nil {
		return err
	}
	//reload alertmanager
	if err = o.reload(); err != nil {
		return err
	}

	return nil

}

func (o *operator) AddRoute(alert *v1beta1.Alert, c *kubernetes.Clientset) error {

	//1. get configuration from secret
	//TODO: should not hardcode the namespace
	sClient := c.CoreV1().Secrets("default")

	//TODO: should not hardcode the secret name
	configSecret, err := sClient.Get("alertmanager-config2", metav1.GetOptions{})
	if err != nil {
		return err
	}
	//TODO: should not hardcode the file name
	configBtyes := configSecret.Data["config.yml"]
	configStr := util.DecodeBase64(string(configBtyes))

	newConfigStr, err := o.addRoute2Config(configStr, alert)
	if err != nil {
		return err
	}

	configSecret.Data["config.yml"] = []byte(newConfigStr)
	_, err = sClient.Update(configSecret)
	if err != nil {
		return err
	}
	//reload alertmanager
	if err = o.reload(); err != nil {
		return err
	}

	return nil

}

func (o *operator) addRoute2Config(configStr string, alert *v1beta1.Alert) (string, error) {
	return "", nil
}

func (o *operator) addReceiver2Config(configStr string, recipient *v1beta1.Recipient, notifier *v1beta1.Notifier) (string, error) {
	config, err := alertconfig.Load(configStr)
	if err != nil {
		return "", err
	}

	//2. add the receiver to the configuration
	//receiver := makeReceiver(recipient, notifier)
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
	if err != nil {
		return "", err
	}

	newconfig := util.EncodeBase64(string(d))
	return newconfig, nil
}

func (o *operator) reload() error {
	//TODO: should not hardcode the url
	Url, err := url.Parse("http://192.168.99.100:31285")
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%v://%v/-/reload", Url.Scheme, Url.Host)

	resp, err := http.Post(url, "text/html", nil)
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}
