package util

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/dispatch"
	"github.com/prometheus/alertmanager/types"

	"github.com/prometheus/common/model"
	"github.com/satori/go.uuid"
	v1beta1 "github.com/zionwu/alertmanager-operator/client/v1beta1"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateUUID() string {
	u1 := uuid.NewV4()
	return fmt.Sprintf("%s", u1)
}

func WaitForCRDReady(listFunc func(opts metav1.ListOptions) (runtime.Object, error)) error {
	err := wait.Poll(3*time.Second, 10*time.Minute, func() (bool, error) {
		_, err := listFunc(metav1.ListOptions{})
		if err != nil {
			if se, ok := err.(*apierrors.StatusError); ok {
				if se.Status().Code == http.StatusNotFound {
					return false, nil
				}
			}
			return false, err
		}
		return true, nil
	})

	return errors.Wrap(err, fmt.Sprintf("timed out waiting for Custom Resoruce"))
}

func NewAlertCustomResourceDefinition() *extensionsobj.CustomResourceDefinition {
	return &extensionsobj.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.AlertName + "." + v1beta1.Group,
		},
		Spec: extensionsobj.CustomResourceDefinitionSpec{
			Group:   v1beta1.Group,
			Version: v1beta1.Version,
			Scope:   extensionsobj.NamespaceScoped,
			Names: extensionsobj.CustomResourceDefinitionNames{
				Plural: v1beta1.AlertName,
				Kind:   v1beta1.AlertsKind,
			},
		},
	}
}

func NewNotifierCustomResourceDefinition() *extensionsobj.CustomResourceDefinition {
	return &extensionsobj.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.NotifierName + "." + v1beta1.Group,
		},
		Spec: extensionsobj.CustomResourceDefinitionSpec{
			Group:   v1beta1.Group,
			Version: v1beta1.Version,
			Scope:   extensionsobj.ClusterScoped,
			Names: extensionsobj.CustomResourceDefinitionNames{
				Plural: v1beta1.NotifierName,
				Kind:   v1beta1.NotifiersKind,
			},
		},
	}
}

func NewRecipientCustomResourceDefinition() *extensionsobj.CustomResourceDefinition {
	return &extensionsobj.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.RecipientName + "." + v1beta1.Group,
		},
		Spec: extensionsobj.CustomResourceDefinitionSpec{
			Group:   v1beta1.Group,
			Version: v1beta1.Version,
			Scope:   extensionsobj.NamespaceScoped,
			Names: extensionsobj.CustomResourceDefinitionNames{
				Plural: v1beta1.RecipientName,
				Kind:   v1beta1.RecipientsKind,
			},
		},
	}
}

func ReloadConfiguration(url string) error {
	//TODO: what is the wait time
	time.Sleep(10 * time.Second)
	resp, err := http.Post(url+"/-/reload", "text/html", nil)
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

func GetActiveAlertListFromAlertManager(url string) ([]*dispatch.APIAlert, error) {

	res := struct {
		Data   []*dispatch.APIAlert `json:"data"`
		Status string               `json:"status"`
	}{}

	req, err := http.NewRequest(http.MethodGet, url+"/api/v1/alerts", nil)
	if err != nil {
		return nil, err
	}
	//q := req.URL.Query()
	//q.Add("filter", fmt.Sprintf("{%s}", filter))

	client := &http.Client{}
	resp, err := client.Do(req)
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

func SendAlert(url string, alert *v1beta1.Alert) error {
	alertList := model.Alerts{}

	a := &model.Alert{}
	a.Labels = map[model.LabelName]model.LabelValue{}
	a.Labels[model.LabelName("namespace")] = model.LabelValue(alert.Namespace)
	a.Labels[model.LabelName("alert_id")] = model.LabelValue(alert.Name)
	a.Labels[model.LabelName("severity")] = model.LabelValue(alert.Severity)
	a.Labels[model.LabelName("target_id")] = model.LabelValue(alert.TargetID)
	a.Labels[model.LabelName("target_type")] = model.LabelValue(alert.TargetType)
	a.Labels[model.LabelName("description")] = model.LabelValue(alert.Description)

	a.Annotations = map[model.LabelName]model.LabelValue{}
	a.Annotations[model.LabelName("description")] = model.LabelValue(alert.Description)

	alertList = append(alertList, a)

	alertData, err := json.Marshal(alertList)
	if err != nil {
		return err
	}

	resp, err := http.Post(url+"/api/alerts", "application/json", bytes.NewBuffer(alertData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logrus.Debugf("send alert: %s", string(res))

	return nil
}

func AddSilence(url string, alert *v1beta1.Alert) error {

	matchers := []*model.Matcher{}
	m1 := &model.Matcher{
		Name:    "alert_id",
		Value:   alert.Name,
		IsRegex: false,
	}
	matchers = append(matchers, m1)

	m2 := &model.Matcher{
		Name:    "namespace",
		Value:   alert.Namespace,
		IsRegex: false,
	}
	matchers = append(matchers, m2)

	now := time.Now()
	endsAt := now.AddDate(100, 0, 0)
	silence := model.Silence{
		Matchers:  matchers,
		StartsAt:  now,
		EndsAt:    endsAt,
		CreatedAt: now,
		CreatedBy: "rancherlabs",
		Comment:   "silence",
	}

	silenceData, err := json.Marshal(silence)
	if err != nil {
		return err
	}
	logrus.Debugf(string(silenceData))

	resp, err := http.Post(url+"/api/v1/silences", "application/json", bytes.NewBuffer(silenceData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logrus.Debugf("add silence: %s", string(res))

	return nil

}

func RemoveSilence(url string, alert *v1beta1.Alert) error {

	res := struct {
		Data   []*types.Silence `json:"data"`
		Status string           `json:"status"`
	}{}

	req, err := http.NewRequest(http.MethodGet, url+"/api/v1/silences", nil)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	q.Add("filter", fmt.Sprintf("{%s, %s}", "alert_id="+alert.Name, "namespace="+alert.Namespace))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	requestBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(requestBytes, &res); err != nil {
		return err
	}

	if res.Status != "success" {
		return fmt.Errorf("Failed to get silence rules for alert")
	}

	for _, s := range res.Data {
		if s.Status.State == types.SilenceStateActive {
			delReq, err := http.NewRequest(http.MethodDelete, url+"/api/v1/silence/"+s.ID, nil)
			if err != nil {
				return err
			}

			delResp, err := client.Do(delReq)
			defer delResp.Body.Close()

			res, err := ioutil.ReadAll(delResp.Body)
			if err != nil {
				return err
			}
			logrus.Debugf("delete silence: %s", string(res))

		}

	}

	return nil
}

func GetState(alert *v1beta1.Alert, apiAlerts []*dispatch.APIAlert) (string, *dispatch.APIAlert) {

	for _, a := range apiAlerts {
		if string(a.Labels["alert_id"]) == alert.Name && string(a.Labels["namespace"]) == alert.Namespace {
			if a.Status.State == types.AlertStateSuppressed {
				return v1beta1.AlertStateSuppressed, a
			} else {
				return v1beta1.AlertStateActive, a
			}
		}
	}

	return v1beta1.AlertStateEnabled, nil

}

func ValidateSlack(config *v1beta1.SlackConfigSpec) error {
	req := struct {
		Text string `json:"text"`
	}{}

	req.Text = "webhook validation"

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(string(config.SlackApiUrl), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status code is not 200")
	}

	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if string(res) != "ok" {
		return fmt.Errorf("http response is not ok")
	}

	return nil

}

func ValidatePagerDuty(config *v1beta1.EmailConfigSpec) error {

	return nil
}

func ValidateEmail(config *v1beta1.EmailConfigSpec) error {
	// We need to know the hostname for both auth and TLS.
	var c *smtp.Client
	host, port, err := net.SplitHostPort(config.SMTPSmartHost)
	if err != nil {
		return fmt.Errorf("invalid address: %s", err)
	}

	if port == "465" || port == "587" {
		conn, err := tls.Dial("tcp", config.SMTPSmartHost, &tls.Config{ServerName: host})
		if err != nil {
			return err
		}
		c, err = smtp.NewClient(conn, config.SMTPSmartHost)
		if err != nil {
			return err
		}

	} else {
		// Connect to the SMTP smarthost.
		c, err = smtp.Dial(config.SMTPSmartHost)
		if err != nil {
			return err
		}
	}
	defer c.Quit()

	/*
		if config.SMTPRequireTLS {
			if ok, _ := c.Extension("STARTTLS"); !ok {
				return fmt.Errorf("require_tls: true (default), but %q does not advertise the STARTTLS extension", config.SMTPSmartHost)
			}
			tlsConf := &tls.Config{ServerName: host}
			if err := c.StartTLS(tlsConf); err != nil {
				return fmt.Errorf("starttls failed: %s", err)
			}
		}
	*/

	if ok, mech := c.Extension("AUTH"); ok {
		logrus.Debugf("smtp auth type: %s", mech)
		auth, err := auth(mech, config)
		if err != nil {
			return err
		}
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("%T failed: %s", auth, err)
			}
		}
	}

	/*  comment this as the smtp_from is the same as username, not nessary to validate

	data := "smtp server configuation validation"

	from := config.SMTPFrom
	to := config.SMTPFrom

	if err != nil {
		return err
	}

	addrs, err := mail.ParseAddressList(from)
	if err != nil {
		return fmt.Errorf("parsing from addresses: %s", err)
	}
	if len(addrs) != 1 {
		return fmt.Errorf("must be exactly one from address")
	}
	if err := c.Mail(addrs[0].Address); err != nil {
		return fmt.Errorf("sending mail from: %s", err)
	}
	addrs, err = mail.ParseAddressList(to)
	if err != nil {
		return fmt.Errorf("parsing to addresses: %s", err)
	}
	for _, addr := range addrs {
		if err := c.Rcpt(addr.Address); err != nil {
			return fmt.Errorf("sending rcpt to: %s", err)
		}
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		return err
	}
	defer wc.Close()

	buffer := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(buffer)

	fmt.Fprintf(wc, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(wc, "Content-Type: multipart/alternative;  boundary=%s\r\n", multipartWriter.Boundary())
	fmt.Fprintf(wc, "MIME-Version: 1.0\r\n")

	// TODO: Add some useful headers here, such as URL of the alertmanager
	// and active/resolved.
	fmt.Fprintf(wc, "\r\n")

	if len(data) > 0 {
		// Text template
		w, err := multipartWriter.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=UTF-8"}})
		if err != nil {
			return fmt.Errorf("creating part for text template: %s", err)
		}

		_, err = w.Write([]byte(data))
		if err != nil {
			return err
		}
	}

	multipartWriter.Close()
	wc.Write(buffer.Bytes())
	*/
	return nil
}

func auth(mechs string, config *v1beta1.EmailConfigSpec) (smtp.Auth, error) {
	username := config.SMTPAuthUserName

	for _, mech := range strings.Split(mechs, " ") {
		switch mech {
		/*
			case "CRAM-MD5":
				secret := string(config.SMTPAuthSecret)
				if secret == "" {
					continue
				}
				return smtp.CRAMMD5Auth(username, secret), nil

			case "PLAIN":
				password := string(config.SMTPAuthPassword)
				if password == "" {
					continue
				}
				identity := config.SMTPAuthIdentity

				// We need to know the hostname for both auth and TLS.
				host, _, err := net.SplitHostPort(config.SMTPSmartHost)
				if err != nil {
					return nil, fmt.Errorf("invalid address: %s", err)
				}
				return smtp.PlainAuth(identity, username, password, host), nil
		*/
		case "LOGIN":
			password := string(string(config.SMTPAuthPassword))
			if password == "" {
				continue
			}

			return &loginAuth{username, password}, nil
		}
	}
	return nil, fmt.Errorf("smtp server does not support login auth")
}

type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

// Used for AUTH LOGIN. (Maybe password should be encrypted)
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch strings.ToLower(string(fromServer)) {
		case "username:":
			return []byte(a.username), nil
		case "password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("unexpected server challenge")
		}
	}
	return nil, nil
}
