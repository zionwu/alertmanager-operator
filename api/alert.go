package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/zionwu/alertmanager-operator/util"
)

func (s *Server) AlertsList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list alert")
	}()

	opt := metav1.ListOptions{}
	l, err := s.AlertClient.List(opt)

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "alert"
	resp.CreateTypes = map[string]string{
		"alert": apiContext.UrlBuilder.Collection("alert"),
	}

	data := []interface{}{}
	for _, item := range l.Items {
		rn := toAlertResource(apiContext, item)
		data = append(data, rn)
	}
	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) CreateAlert(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to create alert")
	}()

	apiContext := api.GetApiContext(req)
	requestBytes, err := ioutil.ReadAll(req.Body)
	alert := Alert{}

	if err := json.Unmarshal(requestBytes, &alert); err != nil {
		return err
	}

	//TODO: generate name
	alert.Id = util.GenerateUUID()
	n := toAlertCRD(&alert)
	_, err = s.AlertClient.Create(n)

	if err != nil {
		return err
	}

	apiContext.Write(&alert)
	return nil
}

func (s *Server) GetAlert(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	opt := metav1.GetOptions{}
	n, err := s.AlertClient.Get(id, opt)
	if err != nil {
		return err
	}
	rn := toAlertResource(apiContext, n)
	apiContext.WriteResource(rn)
	return nil

}

func (s *Server) DeleteAlert(rw http.ResponseWriter, req *http.Request) (err error) {

	//apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]
	opt := metav1.DeleteOptions{}
	err = s.AlertClient.Delete(id, &opt)
	if err != nil {
		return err
	}
	return nil

}

func (s *Server) UpdateAlert(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	requestBytes, err := ioutil.ReadAll(req.Body)
	alert := Alert{}

	if err := json.Unmarshal(requestBytes, &alert); err != nil {
		return err
	}
	alert.Id = id
	n := toAlertCRD(&alert)
	_, err = s.AlertClient.Update(n)

	if err != nil {
		return err
	}

	apiContext.Write(&alert)
	return nil

}
