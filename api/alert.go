package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/dispatch"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/zionwu/alertmanager-operator/alertmanager"
	"github.com/zionwu/alertmanager-operator/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *Server) alertsList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list alert")
	}()

	opt := metav1.ListOptions{}
	l, err := s.alertClient.List(opt)
	if err != nil {
		logrus.Errorf("Error while listing k8s alert CRD: %v", err)
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "alert"
	resp.CreateTypes = map[string]string{
		"alert": apiContext.UrlBuilder.Collection("alert"),
	}

	//TODO:should get the env from request
	activeAlerts, err := s.configOperator.GetActiveAlertListFromAlertManager("enviroment=default")
	if err != nil {
		logrus.Errorf("Error while getting active alert: %v", err)
		return err
	}

	data := []interface{}{}
	for _, item := range l.Items {
		rn := toAlertResource(apiContext, item)
		setAlertState(rn, activeAlerts)
		data = append(data, rn)

	}
	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func setAlertState(item *Alert, activeAlerts []*dispatch.APIAlert) {

	for _, alert := range activeAlerts {
		if string(alert.Labels[alertmanager.AlertIDLabelName]) == item.Id {
			item.State = "active"
			return
		}
	}
}

func (s *Server) createAlert(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to create alert")
	}()

	apiContext := api.GetApiContext(req)
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Errorf("Error while reading request body: %v", err)
		return err
	}
	alert := Alert{}

	if err := json.Unmarshal(requestBytes, &alert); err != nil {
		logrus.Errorf("Error while unmarshal the request: %v", err)
		return err
	}

	if err = s.checkAlertParam(&alert); err != nil {
		return err
	}

	//check if the recipient exists
	_, err = s.recipientClient.Get(alert.RecipientID, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while geting the recipient CRD: %v", err)
		return errors.Wrap(err, "unable to find the recipient")
	}

	alert.Id = util.GenerateUUID()
	//TODO: get env from request
	env := "default"
	n := toAlertCRD(&alert, env)
	alertCRD, err := s.alertClient.Create(n)
	if err != nil {
		logrus.Errorf("Error while creating k8s CRD: %v", err)
		return err
	}

	//make change to configuration of alert manager
	if err = s.configOperator.AddRoute(alertCRD); err != nil {
		logrus.Errorf("Error while adding route config: %v", err)
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
		logrus.Errorf("Error while getting k8s alert CRD: %v", err)
		return err
	}

	//TODO:should get the env from request
	activeAlerts, err := s.configOperator.GetActiveAlertListFromAlertManager("alert_id=" + n.Name)
	if err != nil {
		logrus.Errorf("Error while getting active alert: %v", err)
		return err
	}
	rn := toAlertResource(apiContext, n)
	setAlertState(rn, activeAlerts)

	apiContext.WriteResource(rn)
	return nil

}

func (s *Server) deleteAlert(rw http.ResponseWriter, req *http.Request) (err error) {

	//apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]
	opt := metav1.DeleteOptions{}
	err = s.alertClient.Delete(id, &opt)
	if err != nil {
		logrus.Errorf("Error while deleting k8s alert CRD", err)
		return err
	}

	//TODO: delete route in configuration of alert manager

	return nil

}

func (s *Server) updateAlert(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Errorf("Error while reading request: %v", err)
		return err
	}

	alert := Alert{}

	if err := json.Unmarshal(requestBytes, &alert); err != nil {
		logrus.Errorf("Error while unmarshal the request: %v", err)
		return err
	}

	if err = s.checkAlertParam(&alert); err != nil {
		return err
	}

	//check if the recipient exists
	_, err = s.recipientClient.Get(alert.RecipientID, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while geting the recipient CRD: %v", err)
		return errors.Wrap(err, "unable to find the recipient")
	}

	alert.Id = id
	//TODO: get env from request
	env := "default"
	n := toAlertCRD(&alert, env)
	_, err = s.alertClient.Update(n)

	if err != nil {
		logrus.Errorf("Error while updating k8s alert CRD", err)
		return err
	}

	//update the route in configuration of alert manager

	apiContext.Write(&alert)
	return nil

}

//TODO: check all enum field valid
func (s *Server) checkAlertParam(alert *Alert) error {

	if alert.Name == "" {
		return fmt.Errorf("missing name")
	}

	if alert.RecipientID == "" {
		return fmt.Errorf("missing RecipientID")
	}

	if alert.Object == "" {
		return fmt.Errorf("missing Object")
	}

	if alert.ObjectID == "" {
		return fmt.Errorf("missing ObjectID")
	}

	if alert.ServiceRule.UnhealthyPercetage == "" {
		return fmt.Errorf("missing percentage")
	}

	return nil
}
