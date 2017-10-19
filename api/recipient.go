package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/zionwu/alertmanager-operator/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *Server) recipientsList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list recipient")
	}()

	opt := metav1.ListOptions{}
	l, err := s.recipientClient.List(opt)

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
	//TODO: check if the corresponding notifier exists

	apiContext := api.GetApiContext(req)
	requestBytes, err := ioutil.ReadAll(req.Body)
	recipient := Recipient{}

	if err := json.Unmarshal(requestBytes, &recipient); err != nil {
		return err
	}

	recipient.Id = util.GenerateUUID()
	//TODO: get env from request
	environment := "default"
	n := toRecipientCRD(&recipient, environment)
	_, err = s.recipientClient.Create(n)
	recipientCRD, err := s.recipientClient.Create(n)

	if err != nil {
		return err
	}

	//TODO: get the notifier for this recipient
	selector := "environment=" + recipientCRD.Labels["environment"] + "&" + "type=" + recipientCRD.Labels["type"]
	notifierList, err := s.notifierClient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	if len(notifierList.Items) == 0 {
		return fmt.Errorf("can not find notifier for %s", recipient.Type)
	}

	//Change alertmanager configuration
	if err = s.configOperator.AddReceiver(recipientCRD, notifierList.Items[0]); err != nil {
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
		return err
	}

	return nil

}

func (s *Server) updateRecipient(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	requestBytes, err := ioutil.ReadAll(req.Body)
	recipient := Recipient{}

	if err := json.Unmarshal(requestBytes, &recipient); err != nil {
		return err
	}
	recipient.Id = id
	//TODO: get env from request
	env := "default"
	n := toRecipientCRD(&recipient, env)
	_, err = s.recipientClient.Update(n)

	if err != nil {
		return err
	}

	apiContext.Write(&recipient)
	return nil

}
