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

func (s *Server) notifiersList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list notifier")
	}()

	l, err := s.mclient.MonitoringV1().Notifiers().List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing notifier CRD", err)
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "notifier"
	resp.CreateTypes = map[string]string{
		"notifier": apiContext.UrlBuilder.Collection("notifier"),
	}

	notifierList := l.(*v1beta1.NotifierList)
	data := []interface{}{}
	for _, item := range notifierList.Items {
		rn := toNotifierResource(apiContext, item)

		nSecret, err := s.clientset.Core().Secrets("default").Get("rancher-notifier", metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Error while getting notifier secret: %v", err)
			return err
		}

		rn.SlackConfig.SlackApiUrl = v1beta1.Secret(string(nSecret.Data["slackApiUrl"]))
		rn.EmailConfig.SMTPAuthPassword = v1beta1.Secret(string(nSecret.Data["smtpAuthPassword"]))

		data = append(data, rn)
	}
	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) createNotifier(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to create notifier")
	}()

	apiContext := api.GetApiContext(req)
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Error("Error while reading request: %s", err)
	}
	notifier := Notifier{}

	if err := json.Unmarshal(requestBytes, &notifier); err != nil {
		logrus.Error("Error while unmarshal request: %s", err)
		return err
	}

	if err = s.checkNotifierParam(&notifier); err != nil {
		return err
	}

	notifier.Id = util.GenerateUUID()

	data := map[string][]byte{}
	data["smtpAuthPassword"] = []byte(string(notifier.EmailConfig.SMTPAuthPassword))
	data["slackApiUrl"] = []byte(notifier.SlackConfig.SlackApiUrl)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: notifier.Id,
		},
		Data: data,
	}
	_, err = s.clientset.Core().Secrets("default").Create(secret)
	if err != nil {
		logrus.Error("Error while creating secrets for notifier CRD: %s", err)
		return err
	}

	//TODO: get env from request
	n := toNotifierCRD(&notifier)
	_, err = s.mclient.MonitoringV1().Notifiers().Create(n)

	if err != nil {
		logrus.Error("Error while creating notifier CRD: %s", err)
		return err
	}

	apiContext.Write(&notifier)
	return nil

}

func (s *Server) getNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	opt := metav1.GetOptions{}
	n, err := s.mclient.MonitoringV1().Notifiers().Get(id, opt)

	if err != nil {
		logrus.Error("Error while getting notifier CRD: %s", err)
		return err
	}
	rn := toNotifierResource(apiContext, n)
	nSecret, err := s.clientset.Core().Secrets("default").Get("rancher-notifier", metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting notifier secret: %v", err)
		return err
	}

	rn.SlackConfig.SlackApiUrl = v1beta1.Secret(string(nSecret.Data["slackApiUrl"]))
	rn.EmailConfig.SMTPAuthPassword = v1beta1.Secret(string(nSecret.Data["smtpAuthPassword"]))

	apiContext.WriteResource(rn)
	return nil

}

func (s *Server) deleteNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	//TODO: if it is in used, not allow to delete
	id := mux.Vars(req)["id"]
	opt := metav1.DeleteOptions{}

	err = s.clientset.Core().Secrets("default").Delete(id, &opt)
	if err != nil {
		logrus.Error("Error while deleting notifier CRD secret: %s", err)
		return err
	}

	err = s.mclient.MonitoringV1().Notifiers().Delete(id, &opt)
	if err != nil {
		logrus.Error("Error while deleting notifier CRD: %s", err)
		return err
	}

	return nil
}

func (s *Server) updateNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Error("Error while reading request %s", err)
		return err
	}
	notifier := Notifier{}

	if err := json.Unmarshal(requestBytes, &notifier); err != nil {
		logrus.Error("Error while unmarshal request %s", err)
		return err
	}

	if err = s.checkNotifierParam(&notifier); err != nil {
		return err
	}

	data := map[string][]byte{}
	data["smtpAuthPassword"] = []byte(string(notifier.EmailConfig.SMTPAuthPassword))
	data["slackApiUrl"] = []byte(notifier.SlackConfig.SlackApiUrl)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
		},
		Data: data,
	}

	_, err = s.clientset.Core().Secrets("default").Update(secret)
	if err != nil {
		logrus.Error("Error while updating notifier CRD secrets %s", err)
		return err
	}

	notifier.Id = id
	n := toNotifierCRD(&notifier)
	_, err = s.mclient.MonitoringV1().Notifiers().Update(n)
	if err != nil {
		logrus.Error("Error while updating notifier CRD %s", err)
		return err
	}

	apiContext.Write(&notifier)
	return nil
}

func (s *Server) validateNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	var vtype string
	vals := req.URL.Query()
	if nsarr, ok := vals["type"]; ok {
		vtype = nsarr[0]
	}
	//apiContext := api.GetApiContext(req)
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Error("Error while reading request %s", err)
		return err
	}
	notifier := Notifier{}

	if err := json.Unmarshal(requestBytes, &notifier); err != nil {
		logrus.Error("Error while unmarshal request %s", err)
		return err
	}

	if vtype == "slack" {
		if err := util.ValidateSlack(&notifier.SlackConfig); err != nil {
			return fmt.Errorf("failed to validate for slack config: %v", err)
		}
	} else if vtype == "email" {
		if err := util.ValidateEmail(&notifier.EmailConfig); err != nil {
			return fmt.Errorf("failed to validate for email config: %v", err)
		}
	} else {
		return fmt.Errorf("invalid type to validate: %s", vtype)
	}

	return nil
}

func (s *Server) checkNotifierParam(notifier *Notifier) error {

	return nil
}
