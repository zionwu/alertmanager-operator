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

func (s *Server) NotifiersList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list notifier")
	}()

	opt := metav1.ListOptions{}
	l, err := s.NotifierClient.List(opt)

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "notifier"
	resp.CreateTypes = map[string]string{
		"notifier": apiContext.UrlBuilder.Collection("notifier"),
	}

	data := []interface{}{}
	for _, item := range l.Items {
		rn := toNotifierResource(apiContext, item)
		data = append(data, rn)
	}
	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) CreateNotifier(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to create notifier")
	}()

	apiContext := api.GetApiContext(req)
	requestBytes, err := ioutil.ReadAll(req.Body)
	notifier := Notifier{}

	if err := json.Unmarshal(requestBytes, &notifier); err != nil {
		return err
	}

	//TODO: generate name
	notifier.Id = util.GenerateUUID()
	n := toNotifierCRD(&notifier)
	_, err = s.NotifierClient.Create(n)

	if err != nil {
		return err
	}

	apiContext.Write(&notifier)
	return nil

}

func (s *Server) GetNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	opt := metav1.GetOptions{}
	n, err := s.NotifierClient.Get(id, opt)
	
	if err != nil {
		return err
	}
	rn := toNotifierResource(apiContext, n)
	apiContext.WriteResource(rn)
	return nil

}

func (s *Server) DeleteNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	//apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]
	opt := metav1.DeleteOptions{}
	err = s.NotifierClient.Delete(id, &opt)
	if err != nil {
		return err
	}
	return nil

}

func (s *Server) UpdateNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	requestBytes, err := ioutil.ReadAll(req.Body)
	notifier := Notifier{}

	if err := json.Unmarshal(requestBytes, &notifier); err != nil {
		return err
	}
	notifier.Id = id
	n := toNotifierCRD(&notifier)
	_, err = s.NotifierClient.Update(n)

	if err != nil {
		return err
	}

	apiContext.Write(&notifier)
	return nil

}
