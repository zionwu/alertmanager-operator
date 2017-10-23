package api

import (
	"net/http"

	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/zionwu/alertmanager-operator/alertmanager"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sapi "k8s.io/client-go/pkg/api"
)

type Server struct {
	clientset       *kubernetes.Clientset
	mclient         *v1beta1.MonitoringV1Client
	notifierClient  v1beta1.NotifierInterface
	recipientClient v1beta1.RecipientInterface
	alertClient     v1beta1.AlertInterface
	configOperator  alertmanager.Operator
}

type Error struct {
	client.Resource
	Status   int    `json:"status"`
	Code     string `json:"code"`
	Msg      string `json:"message"`
	Detail   string `json:"detail"`
	BaseType string `json:"baseType"`
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
	Name         string                  `json:"name"`
	State        string                  `json:"state"`
	SendResolved bool                    `json:"sendResolved"`
	Severity     string                  `json:"severity"`
	Object       string                  `json:"object"`
	ObjectID     string                  `json:"objectId"`
	ServiceRule  v1beta1.ServiceRuleSpec `json:"serviceRule"`
	RecipientID  string                  `json:"recipientId"`
}

type Recipient struct {
	client.Resource
	RecipientType      string                         `json:"recipientType"`
	SlackRecipient     v1beta1.SlackRecipientSpec     `json:"slackRecipient"`
	EmailRecipient     v1beta1.EmailRecipientSpec     `json:"emailRecipient"`
	PagerDutyRecipient v1beta1.PagerDutyRecipientSpec `json:"pagerdutyRecipient"`
}

func NewServer(clientset *kubernetes.Clientset, mclient *v1beta1.MonitoringV1Client, alertManagerURL string, alertSecretName string, alertmanagerConfig string) *Server {
	//TODO: should not hardcode name space here
	notifierClient := mclient.Notifiers(k8sapi.NamespaceDefault)
	recipientClient := mclient.Recipients(k8sapi.NamespaceDefault)
	alertClient := mclient.Alerts(k8sapi.NamespaceDefault)

	operator := alertmanager.NewOperator(clientset, alertManagerURL, alertSecretName, alertmanagerConfig)

	return &Server{
		clientset:       clientset,
		mclient:         mclient,
		notifierClient:  notifierClient,
		recipientClient: recipientClient,
		alertClient:     alertClient,
		configOperator:  operator,
	}
}

func newSchema() *client.Schemas {
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
	severity.Default = "critical"
	alert.ResourceFields["severity"] = severity

	state := alert.ResourceFields["state"]
	state.Create = false
	state.Update = false
	state.Type = "enum"
	state.Default = "inactive"
	state.Options = []string{"active", "inactive"}
	alert.ResourceFields["state"] = state

	name := alert.ResourceFields["name"]
	name.Create = true
	name.Update = true
	alert.ResourceFields["name"] = name

	sendResolved := alert.ResourceFields["sendResolved"]
	sendResolved.Create = true
	sendResolved.Update = true
	sendResolved.Default = false
	alert.ResourceFields["sendResolved"] = sendResolved

	object := alert.ResourceFields["object"]
	object.Create = true
	object.Update = true
	object.Type = "enum"
	object.Options = []string{"service", "container", "host", "custom"}
	alert.ResourceFields["object"] = object

	objectId := alert.ResourceFields["objectId"]
	objectId.Create = true
	objectId.Update = true
	alert.ResourceFields["objectId"] = objectId

	recipientId := alert.ResourceFields["recipientId"]
	recipientId.Create = true
	recipientId.Update = true
	recipientId.Type = "reference[recipient]"
	alert.ResourceFields["recipientId"] = recipientId

}

func recipientSchema(recipient *client.Schema) {
	//TODO: remove unsued method like post/delete
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
	//TODO: remove unsued method like post/delete
	notifier.CollectionMethods = []string{http.MethodGet, http.MethodPost}
	notifier.ResourceMethods = []string{http.MethodGet, http.MethodPut, http.MethodDelete}

	notifierType := notifier.ResourceFields["notifierType"]
	notifierType.Create = true
	notifierType.Required = true
	notifierType.Update = false
	notifierType.Type = "enum"
	notifierType.Options = []string{"email", "slack", "pagerduty"}
	notifier.ResourceFields["notifierType"] = notifierType
	notifier.ResourceActions = map[string]client.Action{
		"validate": {
			Input:  "notifier",
			Output: "notifier",
		},
	}
}

func toNotifierResource(apiContext *api.ApiContext, n *v1beta1.Notifier) *Notifier {

	rn := &Notifier{}
	rn.Resource = client.Resource{
		Id:      n.Name,
		Type:    "notifier",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	rn.NotifierType = n.Spec.Type
	switch rn.NotifierType {
	case "email":
		rn.EmailConfig = *n.Spec.EmailConfig
	case "slack":
		rn.SlackConfig = *n.Spec.SlackConfig
	case "pagerduty":
		rn.PagerDutyConfig = *n.Spec.PagerDutyConfig
	}

	rn.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("notifier", rn.Id)
	rn.Actions["validate"] = apiContext.UrlBuilder.ReferenceLink(rn.Resource) + "?action=validate"

	return rn
}

func toNotifierCRD(rn *Notifier, env string) *v1beta1.Notifier {
	n := &v1beta1.Notifier{
		ObjectMeta: metav1.ObjectMeta{
			Name: rn.Id,
			Labels: map[string]string{
				"environment": env,
				"type":        rn.NotifierType,
			},
		},
	}

	spec := v1beta1.NotifierSpec{
		Type:            rn.NotifierType,
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

	rn.RecipientType = n.Spec.Type
	switch rn.RecipientType {
	case "email":
		rn.EmailRecipient = *n.Spec.EmailRecipient
	case "slack":
		rn.SlackRecipient = *n.Spec.SlackRecipient
	case "pagerduty":
		rn.PagerDutyRecipient = *n.Spec.PagerDutyRecipient
	}

	rn.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("recipient", rn.Id)

	return rn
}

func toRecipientCRD(rn *Recipient, env string) *v1beta1.Recipient {
	n := &v1beta1.Recipient{
		ObjectMeta: metav1.ObjectMeta{
			Name: rn.Id,
			Labels: map[string]string{
				"environment": env,
				"type":        rn.RecipientType,
			},
		},
	}

	spec := v1beta1.RecipientSpec{
		Type:               rn.RecipientType,
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
		State:        "inactive",
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

	ra.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("alert", ra.Id)
	ra.Resource.Links["remove"] = apiContext.UrlBuilder.ReferenceByIdLink("alert", ra.Id)
	ra.Resource.Links["recipient"] = apiContext.UrlBuilder.ReferenceByIdLink("recipient", ra.RecipientID)

	return ra
}

func toAlertCRD(ra *Alert, env string) *v1beta1.Alert {
	alert := &v1beta1.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name: ra.Id,
			Labels: map[string]string{
				"environment": env,
			},
		},
	}

	//TODO: come up with util method for object transfermation
	spec := v1beta1.AlertSpec{
		Name:         ra.Name,
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
