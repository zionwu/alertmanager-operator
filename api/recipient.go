package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	"github.com/zionwu/alertmanager-operator/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/pkg/api/v1"
)

func (s *Server) recipientsList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list recipient")
	}()

	namespace := metav1.NamespaceAll
	vals := req.URL.Query()
	if nsarr, ok := vals["namespace"]; ok {
		namespace = nsarr[0]
	}

	opt := metav1.ListOptions{}
	l, err := s.mclient.MonitoringV1().Recipients(namespace).List(opt)
	if err != nil {
		logrus.Errorf("Error while listing recipient CRD: %v", err)
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "recipient"
	resp.CreateTypes = map[string]string{
		"recipient": apiContext.UrlBuilder.Collection("recipient"),
	}

	recipientList := l.(*v1beta1.RecipientList)
	data := []interface{}{}
	for _, item := range recipientList.Items {
		rn := toRecipientResource(apiContext, item)
		data = append(data, rn)
	}
	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) createRecipient(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to create recipient")
	}()

	apiContext := api.GetApiContext(req)
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Errorf("Error while reading request: %v", err)
		return err
	}

	recipient := Recipient{}

	if err := json.Unmarshal(requestBytes, &recipient); err != nil {
		logrus.Errorf("Error while unmarshal request: %v", err)
		return err
	}

	//TODO: need to check if the notifier is configured
	if err = s.checkRecipientParam(&recipient); err != nil {
		return err
	}

	recipient.Id = util.GenerateUUID()

	data := map[string][]byte{}
	data["serviceKey"] = []byte(string(recipient.PagerDutyRecipient.ServiceKey))
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: recipient.Id,
		},
		Data: data,
	}

	_, err = s.clientset.Core().Secrets(recipient.Namespace).Create(secret)
	if err != nil {
		logrus.Errorf("Error while creating recipient CRD secret: %v", err)
		return err
	}

	n := toRecipientCRD(&recipient)
	recipientCRD, err := s.mclient.MonitoringV1().Recipients(recipient.Namespace).Create(n)

	if err != nil {
		logrus.Errorf("Error while creating recipient CRD: %v", err)
		return err
	}

	res := toRecipientResource(apiContext, recipientCRD)
	apiContext.Write(res)
	return nil
}

func (s *Server) getRecipient(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	var namespace string
	vals := req.URL.Query()
	if nsarr, ok := vals["namespace"]; ok {
		namespace = nsarr[0]
	}
	if namespace == "" {
		return fmt.Errorf("Namespace should not be empty")
	}

	opt := metav1.GetOptions{}
	n, err := s.mclient.MonitoringV1().Recipients(namespace).Get(id, opt)
	if err != nil {
		logrus.Error("Error while adding receiver: %v", err)
		return err
	}
	rn := toRecipientResource(apiContext, n)
	apiContext.WriteResource(rn)
	return nil

}

func (s *Server) deleteRecipient(rw http.ResponseWriter, req *http.Request) (err error) {

	//apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	var namespace string
	vals := req.URL.Query()
	if nsarr, ok := vals["namespace"]; ok {
		namespace = nsarr[0]
	}
	if namespace == "" {
		return fmt.Errorf("Namespace should not be empty")
	}

	//can not use filed selector for CRD, https://github.com/kubernetes/kubernetes/issues/51046
	// need to filter it ourselves
	l, err := s.mclient.MonitoringV1().Alerts(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	alerts := l.(*v1beta1.AlertList)
	if len(alerts.Items) != 0 {
		for _, item := range alerts.Items {
			if item.RecipientID == id {
				return fmt.Errorf("The recipient %s is still in used", id)
			}
		}
	}

	opt := metav1.DeleteOptions{}

	err = s.clientset.Core().Secrets(namespace).Delete(id, &opt)
	if err != nil {
		logrus.Error("Error while deleting recipient CRD secret: %v", err)
		return err
	}

	err = s.mclient.MonitoringV1().Recipients(namespace).Delete(id, &opt)
	if err != nil {
		logrus.Error("Error while deleting recipient CRD: %v", err)
		return err
	}

	return nil

}

func (s *Server) updateRecipient(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Errorf("Error while reading request: %v", err)
		return err
	}
	recipient := Recipient{}

	if err := json.Unmarshal(requestBytes, &recipient); err != nil {
		logrus.Errorf("Error while unmarshaling request: %v", err)
		return err
	}

	if err = s.checkRecipientParam(&recipient); err != nil {
		return err
	}

	recipient.Id = id

	data := map[string][]byte{}
	data["serviceKey"] = []byte(string(recipient.PagerDutyRecipient.ServiceKey))
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: recipient.Id,
		},
		Data: data,
	}

	_, err = s.clientset.Core().Secrets(recipient.Namespace).Update(secret)
	if err != nil {
		logrus.Errorf("Error while updating recipient CRD secret: %v", err)
		return err
	}

	n := toRecipientCRD(&recipient)
	_, err = s.mclient.MonitoringV1().Recipients(recipient.Namespace).Update(n)

	if err != nil {
		logrus.Error("Error while updating recipient CRD: %v", err)
		return err
	}

	apiContext.Write(&recipient)
	return nil

}

func (s *Server) checkRecipientParam(recipient *Recipient) error {

	recipientType := recipient.RecipientType
	if !(recipientType == "email" || recipientType == "slack" || recipientType == "pagerduty" || recipientType == "webhook") {
		return fmt.Errorf("recipientTpye should be email/slack/pagerduty")
	}

	switch recipientType {
	case "email":
		if recipient.EmailRecipient.Address == "" {
			return fmt.Errorf("email address can't be empty")
		}
	case "slack":
		if recipient.SlackRecipient.Channel == "" {
			return fmt.Errorf("slack channel can't be empty")
		}
	case "pagerduty":
		if recipient.PagerDutyRecipient.ServiceKey == "" {
			return fmt.Errorf("pagerduty servicekey can't be empty")
		}

		if recipient.PagerDutyRecipient.ServiceName == "" {
			return fmt.Errorf("pagerduty service name can't be empty")
		}
	case "webhook":
		if recipient.WebhookRecipient.URL == "" {
			return fmt.Errorf("webhook url can't be empty")
		}

		if recipient.WebhookRecipient.Name == "" {
			return fmt.Errorf("webhook name can't be empty")
		}
	}

	return nil
}
