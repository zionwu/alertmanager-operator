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
)

func (s *Server) alertsList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list alert")
	}()

	namespace := metav1.NamespaceAll
	vals := req.URL.Query()
	if nsarr, ok := vals["namespace"]; ok {
		namespace = nsarr[0]
	}

	l, err := s.mclient.MonitoringV1().Alerts(namespace).List(metav1.ListOptions{})
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

	alertList := l.(*v1beta1.AlertList)
	data := []interface{}{}
	for _, item := range alertList.Items {
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
	if err != nil {
		logrus.Errorf("Error while reading request body: %v", err)
		return err
	}
	alert := Alert{State: v1beta1.AlertStateEnabled}

	if err := json.Unmarshal(requestBytes, &alert); err != nil {
		logrus.Errorf("Error while unmarshal the request: %v", err)
		return err
	}

	if err = s.checkAlertParam(&alert); err != nil {
		return err
	}

	//check if the recipient exists
	_, err = s.mclient.MonitoringV1().Recipients(alert.Namespace).Get(alert.RecipientID, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while geting the recipient CRD: %v", err)
		return errors.Wrap(err, "unable to find the recipient")
	}

	alert.Id = util.GenerateUUID()
	n := toAlertCRD(&alert)
	alertCRD, err := s.mclient.MonitoringV1().Alerts(alert.Namespace).Create(n)
	if err != nil {
		logrus.Errorf("Error while creating k8s CRD: %v", err)
		return err
	}

	res := toAlertResource(apiContext, alertCRD)

	apiContext.Write(res)
	return nil
}

func (s *Server) getAlert(rw http.ResponseWriter, req *http.Request) (err error) {

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

	n, err := s.mclient.MonitoringV1().Alerts(namespace).Get(id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting k8s alert CRD: %v", err)
		return err
	}

	rn := toAlertResource(apiContext, n)

	apiContext.WriteResource(rn)
	return nil

}

func (s *Server) deleteAlert(rw http.ResponseWriter, req *http.Request) (err error) {

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

	alert, err := s.mclient.MonitoringV1().Alerts(namespace).Get(id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting k8s alert CRD: %v", err)
		return err
	}

	if alert.State != v1beta1.AlertStateDisabled {
		return fmt.Errorf("Current state is not inactive, can not perform delete")
	}

	err = s.mclient.MonitoringV1().Alerts(namespace).Delete(id, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("Error while deleting k8s alert CRD", err)
		return err
	}

	ra := toAlertResource(apiContext, alert)
	apiContext.Write(ra)
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
	_, err = s.mclient.MonitoringV1().Recipients(alert.Namespace).Get(alert.RecipientID, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while geting the recipient CRD: %v", err)
		return errors.Wrap(err, "unable to find the recipient")
	}

	alert.Id = id
	//TODO: get env from request
	n := toAlertCRD(&alert)
	_, err = s.mclient.MonitoringV1().Alerts(alert.Namespace).Update(n)

	if err != nil {
		logrus.Errorf("Error while updating k8s alert CRD", err)
		return err
	}

	apiContext.Write(&alert)
	return nil

}

func (s *Server) deactivateAlert(rw http.ResponseWriter, req *http.Request) (err error) {
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

	alert, err := s.mclient.MonitoringV1().Alerts(namespace).Get(id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting k8s alert CRD: %v", err)
		return err
	}

	if alert.State != v1beta1.AlertStateEnabled {
		return fmt.Errorf("Current state is not active, can not perform deactiavte action")
	}

	alert.State = v1beta1.AlertStateDisabled

	_, err = s.mclient.MonitoringV1().Alerts(namespace).Update(alert)
	if err != nil {
		logrus.Errorf("Error while deactivating k8s alert CRD", err)
		return err
	}

	ra := toAlertResource(apiContext, alert)
	apiContext.Write(ra)

	return nil

}

func (s *Server) activateAlert(rw http.ResponseWriter, req *http.Request) (err error) {
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

	alert, err := s.mclient.MonitoringV1().Alerts(namespace).Get(id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting k8s alert CRD: %v", err)
		return err
	}

	if alert.State != v1beta1.AlertStateDisabled {
		return fmt.Errorf("Current state is not inactive, can not perform actiavte action")
	}

	alert.State = v1beta1.AlertStateEnabled

	_, err = s.mclient.MonitoringV1().Alerts(namespace).Update(alert)
	if err != nil {
		logrus.Errorf("Error while setting k8s alert to active", err)
		return err
	}

	ra := toAlertResource(apiContext, alert)
	apiContext.Write(ra)

	return nil

}

func (s *Server) silenceAlert(rw http.ResponseWriter, req *http.Request) (err error) {
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

	alert, err := s.mclient.MonitoringV1().Alerts(namespace).Get(id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting k8s alert CRD: %v", err)
		return err
	}

	if alert.State != v1beta1.AlertStateActive {
		return fmt.Errorf("Current state is not alerting, can not perform slience action")
	}

	err = util.AddSilence(s.cfg.ManagerUrl, alert)
	if err != nil {
		return fmt.Errorf("Error while adding silence to AlertManager: %v", err)
	}

	alert.State = v1beta1.AlertStateSuppressed
	_, err = s.mclient.MonitoringV1().Alerts(namespace).Update(alert)
	if err != nil {
		logrus.Errorf("Error while setting k8s alert to silenced", err)
		return err
	}

	ra := toAlertResource(apiContext, alert)
	apiContext.Write(ra)

	return nil
}

func (s *Server) unsilenceAlert(rw http.ResponseWriter, req *http.Request) (err error) {
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

	alert, err := s.mclient.MonitoringV1().Alerts(namespace).Get(id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting k8s alert CRD: %v", err)
		return err
	}

	if alert.State != v1beta1.AlertStateSuppressed {
		return fmt.Errorf("Current state is not silenced, can not perform unslience action")
	}

	err = util.RemoveSilence(s.cfg.ManagerUrl, alert)
	if err != nil {
		return fmt.Errorf("Error while removing silence to AlertManager: %v", err)
	}

	alert.State = v1beta1.AlertStateActive
	_, err = s.mclient.MonitoringV1().Alerts(namespace).Update(alert)
	if err != nil {
		logrus.Errorf("Error while setting k8s alert to alerting", err)
		return err
	}

	ra := toAlertResource(apiContext, alert)
	apiContext.Write(ra)

	return nil

}

//TODO: check all enum field valid
func (s *Server) checkAlertParam(alert *Alert) error {

	if alert.Description == "" {
		return fmt.Errorf("missing description")
	}

	if alert.RecipientID == "" {
		return fmt.Errorf("missing RecipientID")
	}

	if alert.TargetType == "" {
		return fmt.Errorf("missing TargetType")
	}

	if alert.TargetID == "" {
		return fmt.Errorf("missing TargetType")
	}

	return nil
}
