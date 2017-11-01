package api

import (
	"encoding/json"
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

func (s *Server) notifiersList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list notifier")
	}()

	opt := metav1.ListOptions{}
	l, err := s.notifierClient.List(opt)
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
	//TODO: get env from request
	env := "default"
	n := toNotifierCRD(&notifier, env)
	_, err = s.notifierClient.Create(n)

	if err != nil {
		logrus.Error("Error while creating notifier CRD: %s", err)
		return err
	}

	res := toNotifierResource(apiContext, n)
	apiContext.Write(res)
	return nil

}

func (s *Server) getNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	apiContext := api.GetApiContext(req)

	id := mux.Vars(req)["id"]
	opt := metav1.GetOptions{}
	n, err := s.notifierClient.Get(id, opt)

	if err != nil {
		logrus.Error("Error while getting notifier CRD: %s", err)
		return err
	}
	rn := toNotifierResource(apiContext, n)
	apiContext.WriteResource(rn)
	return nil

}

func (s *Server) deleteNotifier(rw http.ResponseWriter, req *http.Request) (err error) {

	//TODO: if it is in used, not allow to delete
	id := mux.Vars(req)["id"]
	opt := metav1.DeleteOptions{}
	err = s.notifierClient.Delete(id, &opt)
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

	notifier.Id = id
	env := "default"
	n := toNotifierCRD(&notifier, env)
	_, err = s.notifierClient.Update(n)

	if err != nil {
		logrus.Error("Error while updating notifier CRD %s", err)
		return err
	}

	apiContext.Write(&notifier)
	return nil
}

func (s *Server) validateNotifier(rw http.ResponseWriter, req *http.Request) (err error) {
	return nil
}

func (s *Server) checkNotifierParam(notifier *Notifier) error {

	return nil
}
