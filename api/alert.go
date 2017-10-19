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

func (s *Server) alertsList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list alert")
	}()

	opt := metav1.ListOptions{}
	l, err := s.alertClient.List(opt)

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

func (s *Server) createAlert(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to create alert")
	}()

	apiContext := api.GetApiContext(req)
	requestBytes, err := ioutil.ReadAll(req.Body)
	alert := Alert{}

	if err := json.Unmarshal(requestBytes, &alert); err != nil {
		return err
	}

	alert.Id = util.GenerateUUID()
	//TODO: get env from request
	env := "environment"
	n := toAlertCRD(&alert, env)
	alertCRD, err := s.alertClient.Create(n)
	if err != nil {
		return err
	}

	//make change to configuration of alert manager
	if err = s.configOperator.AddRoute(alertCRD); err != nil {
		return err
	}

	apiContext.Write(&alert)
	return nil
}

func (s *Server) getAlert(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	opt := metav1.GetOptions{}
	n, err := s.alertClient.Get(id, opt)
	if err != nil {
		return err
	}
	rn := toAlertResource(apiContext, n)
	apiContext.WriteResource(rn)
	return nil

}

func (s *Server) deleteAlert(rw http.ResponseWriter, req *http.Request) (err error) {

	//apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]
	opt := metav1.DeleteOptions{}
	err = s.alertClient.Delete(id, &opt)
	if err != nil {
		return err
	}

	//TODO: delete route in configuration of alert manager

	return nil

}

func (s *Server) updateAlert(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	requestBytes, err := ioutil.ReadAll(req.Body)
	alert := Alert{}

	if err := json.Unmarshal(requestBytes, &alert); err != nil {
		return err
	}
	alert.Id = id
	//TODO: get env from request
	env := "default"
	n := toAlertCRD(&alert, env)
	_, err = s.alertClient.Update(n)

	if err != nil {
		return err
	}

	apiContext.Write(&alert)
	return nil

}
