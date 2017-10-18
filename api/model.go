package api

import (
	"net/http"

	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sapi "k8s.io/client-go/pkg/api"
)

type Error struct {
	client.Resource
	Status   int    `json:"status"`
	Code     string `json:"code"`
	Msg      string `json:"message"`
	Detail   string `json:"detail"`
	BaseType string `json:"baseType"`
}

type Server struct {
	Clientset       *kubernetes.Clientset
	Mclient         *v1beta1.MonitoringV1Client
	NotifierClient  v1beta1.NotifierInterface
	RecipientClient v1beta1.RecipientInterface
	AlertClient     v1beta1.AlertInterface
}

type Notifier struct {
	client.Resource
	NotifierType    string                      `json:"notifierType"`
	SlackConfig     v1beta1.SlackConfigSpec     `json:"slackConfig"`
	EmailConfig     v1beta1.EmailConfigSpec     `json:"emailConfig"`
	PagerDutyConfig v1beta1.PagerDutyConfigSpec `json:"pagerdutyConfig"`
}

type Alert struct {
	client.Resource
	Name         string                  `json:"name,omitempty"`
	Status       string                  `json:"status,omitempty"`
	SendResolved bool                    `json:"sendResolved,omitempty"`
	Severity     string                  `json:"severity, omitempty"`
	Object       string                  `json:"object, omitempty"`
	ObjectID     string                  `json:"objectId, omitempty"`
	ServiceRule  v1beta1.ServiceRuleSpec `json:"serviceRule, omitempty"`
	RecipientID  string                  `json:"recipientId, omitempty"`
}

type Recipient struct {
	client.Resource
	RecipientType      string                         `json:"recipientType"`
	SlackRecipient     v1beta1.SlackRecipientSpec     `json:"slackRecipient"`
	EmailRecipient     v1beta1.EmailRecipientSpec     `json:"emailRecipient"`
	PagerDutyRecipient v1beta1.PagerDutyRecipientSpec `json:"pagerdutyRecipient"`
}

func NewServer(clientset *kubernetes.Clientset, mclient *v1beta1.MonitoringV1Client) *Server {
	//TODO: hardcode name space here
	notifierClient := mclient.Notifiers(k8sapi.NamespaceDefault)
	recipientClient := mclient.Recipients(k8sapi.NamespaceDefault)
	alertClient := mclient.Alerts(k8sapi.NamespaceDefault)
	return &Server{
		Clientset:       clientset,
		Mclient:         mclient,
		NotifierClient:  notifierClient,
		RecipientClient: recipientClient,
		AlertClient:     alertClient,
	}
}

func NewSchema() *client.Schemas {
	schemas := &client.Schemas{}

	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	schemas.AddType("error", Error{})

	notifierSchema(schemas.AddType("notifier", Notifier{}))
	recipientSchema(schemas.AddType("recipient", Recipient{}))
	alertSchema(schemas.AddType("alert", Alert{}))

	return schemas
}

func alertSchema(alert *client.Schema) {

	alert.CollectionMethods = []string{http.MethodGet, http.MethodPost}
	alert.ResourceMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut}

	severity := alert.ResourceFields["severity"]
	severity.Create = true
	severity.Required = true
	severity.Type = "enum"
	severity.Options = []string{"info", "warnning", "critical"}
	alert.ResourceFields["severity"] = severity

}

func recipientSchema(recipient *client.Schema) {

	recipient.CollectionMethods = []string{http.MethodGet, http.MethodPost}
	recipient.ResourceMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut}

	recipientType := recipient.ResourceFields["recipientType"]
	recipientType.Create = true
	recipientType.Required = true
	recipientType.Type = "enum"
	recipientType.Options = []string{"email", "slack", "pagerduty"}
	recipient.ResourceFields["recipientType"] = recipientType

}

func notifierSchema(notifier *client.Schema) {

	notifier.CollectionMethods = []string{http.MethodGet, http.MethodPost}
	notifier.ResourceMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut}

	notifierType := notifier.ResourceFields["notifierType"]
	notifierType.Create = true
	notifierType.Required = true
	notifierType.Update = false
	notifierType.Type = "enum"
	notifierType.Options = []string{"email", "slack", "pagerduty"}
	notifier.ResourceFields["notifierType"] = notifierType

}

func toNotifierResource(apiContext *api.ApiContext, n *v1beta1.Notifier) *Notifier {

	rn := &Notifier{}
	rn.Resource = client.Resource{
		Id:      n.Name,
		Type:    "notifier",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	rn.NotifierType = n.Spec.Kind
	switch rn.NotifierType {
	case "email":
		rn.EmailConfig = *n.Spec.EmailConfig
	case "slack":
		rn.SlackConfig = *n.Spec.SlackConfig
	case "pagerduty":
		rn.PagerDutyConfig = *n.Spec.PagerDutyConfig
	}

	rn.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("notifier", rn.Id)

	return rn
}

func toNotifierCRD(rn *Notifier) *v1beta1.Notifier {
	n := &v1beta1.Notifier{
		ObjectMeta: metav1.ObjectMeta{
			Name: rn.Id,
		},
	}

	spec := v1beta1.NotifierSpec{
		Kind:            rn.NotifierType,
		EmailConfig:     &rn.EmailConfig,
		SlackConfig:     &rn.SlackConfig,
		PagerDutyConfig: &rn.PagerDutyConfig,
	}

	n.Spec = spec
	return n
}

func toRecipientResource(apiContext *api.ApiContext, n *v1beta1.Recipient) *Recipient {

	rn := &Recipient{}
	rn.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      n.Name,
		Type:    "recipient",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	rn.RecipientType = n.Spec.Kind
	switch rn.RecipientType {
	case "email":
		rn.EmailRecipient = *n.Spec.EmailRecipient
	case "slack":
		rn.SlackRecipient = *n.Spec.SlackRecipient
	case "pagerduty":
		rn.PagerDutyRecipient = *n.Spec.PagerDutyRecipient
	}

	return rn
}

func toRecipientCRD(rn *Recipient) *v1beta1.Recipient {
	n := &v1beta1.Recipient{
		ObjectMeta: metav1.ObjectMeta{
			Name: rn.Id,
		},
	}

	spec := v1beta1.RecipientSpec{
		Kind:               rn.RecipientType,
		EmailRecipient:     &rn.EmailRecipient,
		SlackRecipient:     &rn.SlackRecipient,
		PagerDutyRecipient: &rn.PagerDutyRecipient,
	}

	n.Spec = spec
	return n
}

func toAlertResource(apiContext *api.ApiContext, a *v1beta1.Alert) *Alert {
	ra := &Alert{
		Name:         a.Spec.Name,
		Status:       a.Spec.Status,
		SendResolved: a.Spec.SendResolved,
		Severity:     a.Spec.Severity,
		Object:       a.Spec.Object,
		ObjectID:     a.Spec.ObjectID,
		ServiceRule:  a.Spec.ServiceRule,
		RecipientID:  a.Spec.RecipientID,
	}
	ra.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      a.Name,
		Type:    "alert",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	return ra
}

func toAlertCRD(ra *Alert) *v1beta1.Alert {
	alert := &v1beta1.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name: ra.Id,
		},
	}

	//TODO: come up with util method for object transfermation
	spec := v1beta1.AlertSpec{
		Name:         ra.Name,
		Status:       ra.Status,
		SendResolved: ra.SendResolved,
		Severity:     ra.Severity,
		Object:       ra.Object,
		ObjectID:     ra.ObjectID,
		ServiceRule:  ra.ServiceRule,
		RecipientID:  ra.RecipientID,
	}
	alert.Spec = spec
	return alert
}
