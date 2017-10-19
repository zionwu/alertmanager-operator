package api

import (
	"encoding/json"
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
	l, err := s.RecipientClient.List(opt)

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

	//TODO: generate name
	recipient.Id = util.GenerateUUID()
	n := toRecipientCRD(&recipient)
	_, err = s.RecipientClient.Create(n)
	//recipientCRD, err := s.RecipientClient.Create(n)

	if err != nil {
		return err
	}

	//TODO: get the notifier for this recipient
	//notifier := v1beta1.Notifier{}
	//Change alertmanager configuration
	//alertmanager.AddReceiver(recipientCRD, notifier, s.Clientset)

	apiContext.Write(&recipient)
	return nil

}

func (s *Server) getRecipient(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	opt := metav1.GetOptions{}
	n, err := s.RecipientClient.Get(id, opt)
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
	err = s.RecipientClient.Delete(id, &opt)
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
	n := toRecipientCRD(&recipient)
	_, err = s.RecipientClient.Update(n)

	if err != nil {
		return err
	}

	apiContext.Write(&recipient)
	return nil

}
