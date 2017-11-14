package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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

func GetState(alert *v1beta1.Alert, apiAlerts []*dispatch.APIAlert) string {

	for _, a := range apiAlerts {
		if string(a.Labels["alert_id"]) == alert.Name && string(a.Labels["namespace"]) == alert.Namespace {
			if a.Status.State == types.AlertStateSuppressed {
				return v1beta1.AlertStateSilenced
			} else {
				return v1beta1.AlertStateAlerting
			}
		}
	}

	return v1beta1.AlertStateActive

}
