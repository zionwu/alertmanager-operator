package api

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
	appv1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
	ev1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type Pod struct {
	client.Resource
}

type Node struct {
	client.Resource
}

type Deployment struct {
	client.Resource
}

type StatefulSet struct {
	client.Resource
}

type DaemonSet struct {
	client.Resource
}

type Namespace struct {
	client.Resource
	Name      string `json:"name"`
	ClusterID string `json:"clusterId"`
}

func namespaceSchema(ns *client.Schema) {
	ns.CollectionMethods = []string{http.MethodGet}
}

func toNamespaceResource(apiContext *api.ApiContext, ns *v1.Namespace) *Namespace {
	ra := &Namespace{}
	ra.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      ns.Name,
		Type:    "namespace",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	ra.Name = ns.Name
	ra.ClusterID = "minikube"
	return ra
}

func nodeSchema(node *client.Schema) {
	node.CollectionMethods = []string{http.MethodGet}
}

func toNodeResource(apiContext *api.ApiContext, node *v1.Node) *Node {
	ra := &Node{}
	ra.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      node.Name,
		Type:    "node",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	return ra
}

func statefulsetSchema(statefulset *client.Schema) {
	statefulset.CollectionMethods = []string{http.MethodGet}
}

func toStatefulsetResource(apiContext *api.ApiContext, statefulSet *appv1beta1.StatefulSet) *StatefulSet {
	ra := &StatefulSet{}
	ra.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      statefulSet.Name,
		Type:    "statefulset",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	return ra
}

func daemonsetSchema(daemonSet *client.Schema) {
	daemonSet.CollectionMethods = []string{http.MethodGet}
}

func toDaemonsetResource(apiContext *api.ApiContext, daemonSet *ev1beta1.DaemonSet) *DaemonSet {
	ra := &DaemonSet{}
	ra.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      daemonSet.Name,
		Type:    "daemonset",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	return ra
}

func deploymentSchema(deployment *client.Schema) {
	deployment.CollectionMethods = []string{http.MethodGet}
}

func toDeploymentResource(apiContext *api.ApiContext, deployment *appv1beta1.Deployment) *Deployment {
	ra := &Deployment{}
	ra.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      deployment.Name,
		Type:    "deployment",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	return ra
}

func podSchema(pod *client.Schema) {
	pod.CollectionMethods = []string{http.MethodGet}
}

func toPodResource(apiContext *api.ApiContext, pod *v1.Pod) *Pod {
	ra := &Pod{}
	ra.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      pod.Name,
		Type:    "pod",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	return ra
}

func (s *Server) podList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list pod")
	}()

	podList, err := s.clientset.CoreV1().Pods("default").List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing k8s pods: %v", err)
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "pod"
	resp.CreateTypes = map[string]string{
		"pod": apiContext.UrlBuilder.Collection("pod"),
	}

	data := []interface{}{}
	for _, item := range podList.Items {
		rn := toPodResource(apiContext, &item)
		data = append(data, rn)

	}
	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) nodeList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list node")
	}()

	nodeList, err := s.clientset.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing k8s nodes: %v", err)
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "node"
	resp.CreateTypes = map[string]string{
		"node": apiContext.UrlBuilder.Collection("node"),
	}

	data := []interface{}{}
	for _, item := range nodeList.Items {
		rn := toNodeResource(apiContext, &item)
		data = append(data, rn)

	}
	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) deploymentList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list deployment")
	}()

	deploymentList, err := s.clientset.Apps().Deployments("default").List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing k8s deployment: %v", err)
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "deployment"
	resp.CreateTypes = map[string]string{
		"deployment": apiContext.UrlBuilder.Collection("deployment"),
	}

	data := []interface{}{}
	for _, item := range deploymentList.Items {
		rn := toDeploymentResource(apiContext, &item)
		data = append(data, rn)
	}

	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) namespaceList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list ns")
	}()

	nsList, err := s.clientset.Core().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing k8s ns: %v", err)
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "namespace"
	resp.CreateTypes = map[string]string{
		"namespace": apiContext.UrlBuilder.Collection("namespace"),
	}

	data := []interface{}{}
	for _, item := range nsList.Items {
		rn := toNamespaceResource(apiContext, &item)
		data = append(data, rn)
	}

	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) daemonsetList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list ds")
	}()

	dsList, err := s.clientset.Extensions().DaemonSets("default").List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing k8s ds: %v", err)
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "daemonset"
	resp.CreateTypes = map[string]string{
		"daemonset": apiContext.UrlBuilder.Collection("daemonset"),
	}

	data := []interface{}{}
	for _, item := range dsList.Items {
		rn := toDaemonsetResource(apiContext, &item)
		data = append(data, rn)
	}

	resp.Data = data
	apiContext.Write(resp)

	return nil
}

func (s *Server) statefulsetList(rw http.ResponseWriter, req *http.Request) (err error) {
	defer func() {
		err = errors.Wrap(err, "unable to list ss")
	}()

	ssList, err := s.clientset.Apps().StatefulSets("default").List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error while listing k8s ss: %v", err)
		return err
	}

	apiContext := api.GetApiContext(req)
	resp := &client.GenericCollection{}

	resp.ResourceType = "statefulset"
	resp.CreateTypes = map[string]string{
		"statefulset": apiContext.UrlBuilder.Collection("statefulset"),
	}

	data := []interface{}{}
	for _, item := range ssList.Items {
		rn := toStatefulsetResource(apiContext, &item)
		data = append(data, rn)
	}

	resp.Data = data
	apiContext.Write(resp)

	return nil
}
