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
	"github.com/zionwu/alertmanager-operator/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

func (s *Server) recipientsList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list recipient")
	}()

	opt := metav1.ListOptions{}
	l, err := s.recipientClient.List(opt)
	if err != nil {
		logrus.Errorf("Error while listing recipient CRD: %v", err)
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "recipient"
	resp.CreateTypes = map[string]string{
		"recipient": apiContext.UrlBuilder.Collection("recipient"),
	}

	data := []interface{}{}
	for _, item := range l.Items {
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

	if err = s.checkRecipientParam(&recipient); err != nil {
		return err
	}

	recipient.Id = util.GenerateUUID()
	//TODO: get env from request
	environment := "default"
	n := toRecipientCRD(&recipient, environment)
	recipientCRD, err := s.recipientClient.Create(n)

	if err != nil {
		logrus.Errorf("Error while creating recipient CRD: %v", err)
		return err
	}

	opt := metav1.ListOptions{
		LabelSelector: fields.SelectorFromSet(fields.Set(map[string]string{
			"environment": recipientCRD.Labels["environment"],
			"type":        recipientCRD.Labels["type"],
		})).String()}
	notifierList, err := s.notifierClient.List(opt)
	if err != nil {
		logrus.Errorf("Error while listing notifier CRD: %v", err)
		return err
	}
	if len(notifierList.Items) == 0 {
		return fmt.Errorf("can not find notifier for %s", recipient.Type)
	}

	//Change alertmanager configuration
	if err = s.configOperator.AddReceiver(recipientCRD, notifierList.Items[0]); err != nil {
		logrus.Error("Error while adding receiver")
		return err
	}

	apiContext.Write(&recipient)
	return nil

}

func (s *Server) getRecipient(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	opt := metav1.GetOptions{}
	n, err := s.recipientClient.Get(id, opt)
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
	opt := metav1.DeleteOptions{}
	err = s.recipientClient.Delete(id, &opt)
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
		logrus.Error("Error while reading request: %v", err)
		return err
	}
	recipient := Recipient{}

	if err := json.Unmarshal(requestBytes, &recipient); err != nil {
		logrus.Error("Error while unmarshaling request: %v", err)
		return err
	}

	if err = s.checkRecipientParam(&recipient); err != nil {
		return err
	}

	recipient.Id = id
	//TODO: get env from request
	env := "default"
	n := toRecipientCRD(&recipient, env)
	_, err = s.recipientClient.Update(n)

	if err != nil {
		logrus.Error("Error while updating recipient CRD: %v", err)
		return err
	}

	apiContext.Write(&recipient)
	return nil

}

func (s *Server) checkRecipientParam(recipient *Recipient) error {
	if recipient.RecipientType == "" {
		return fmt.Errorf("missing recipientType")
	}

	if !(recipient.RecipientType == "email" || recipient.RecipientType == "slack" || recipient.RecipientType == "pagerduty") {
		return fmt.Errorf("not valid value for recipientType")
	}

	switch recipient.RecipientType {
	case "email":
		if recipient.EmailRecipient.Address == "" {
			return fmt.Errorf("missing Address")
		}
	case "slack":
		if recipient.SlackRecipient.Channel == "" {
			return fmt.Errorf("missing channel")
		}
	case "pagerduty":
		if recipient.PagerDutyRecipient.ServiceKey == "" {
			return fmt.Errorf("missing service key")
		}
	}
	return nil
}
