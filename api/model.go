package api

import (
	"net/http"

	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sapi "k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
)

type Server struct {
	clientset       *kubernetes.Clientset
	mclient         *v1beta1.MonitoringV1Client
	notifierClient  v1beta1.NotifierInterface
	recipientClient v1beta1.RecipientInterface
	alertClient     v1beta1.AlertInterface
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
	SlackConfig     v1beta1.SlackConfigSpec     `json:"slackConfig"`
	EmailConfig     v1beta1.EmailConfigSpec     `json:"emailConfig"`
	PagerDutyConfig v1beta1.PagerDutyConfigSpec `json:"pagerdutyConfig"`
}

type Pod struct {
	client.Resource
}

type Alert struct {
	client.Resource
	Description  string                  `json:"description"`
	State        string                  `json:"state"`
	SendResolved bool                    `json:"sendResolved"`
	Severity     string                  `json:"severity"`
	ObjectType   string                  `json:"objectType"`
	ObjectID     string                  `json:"objectId"`
	ServiceRule  v1beta1.ServiceRuleSpec `json:"serviceRule"`
	RecipientID  string                  `json:"recipientId"`
}

type Recipient struct {
	client.Resource
	SlackRecipient     v1beta1.SlackRecipientSpec     `json:"slackRecipient"`
	EmailRecipient     v1beta1.EmailRecipientSpec     `json:"emailRecipient"`
	PagerDutyRecipient v1beta1.PagerDutyRecipientSpec `json:"pagerdutyRecipient"`
}

func NewServer(config *rest.Config) *Server {

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	mclient, err := v1beta1.NewForConfig(config)
	//TODO: should not hardcode name space here
	notifierClient := mclient.Notifiers(k8sapi.NamespaceDefault)
	recipientClient := mclient.Recipients(k8sapi.NamespaceDefault)
	alertClient := mclient.Alerts(k8sapi.NamespaceDefault)

	return &Server{
		clientset:       clientset,
		mclient:         mclient,
		notifierClient:  notifierClient,
		recipientClient: recipientClient,
		alertClient:     alertClient,
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
	podSchema(schemas.AddType("pod", Pod{}))

	return schemas
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

	description := alert.ResourceFields["description"]
	description.Create = true
	description.Update = true
	alert.ResourceFields["description"] = description

	sendResolved := alert.ResourceFields["sendResolved"]
	sendResolved.Create = true
	sendResolved.Update = true
	sendResolved.Default = false
	alert.ResourceFields["sendResolved"] = sendResolved

	objectType := alert.ResourceFields["objectType"]
	objectType.Create = true
	objectType.Update = true
	objectType.Type = "enum"
	objectType.Options = []string{"service", "pod", "host", "custom"}
	alert.ResourceFields["objectType"] = objectType

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
}

func notifierSchema(notifier *client.Schema) {
	//TODO: remove unsued method like post/delete
	notifier.CollectionMethods = []string{http.MethodGet, http.MethodPost}
	notifier.ResourceMethods = []string{http.MethodGet, http.MethodPut, http.MethodDelete}

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

	rn.EmailConfig = *n.EmailConfig
	rn.SlackConfig = *n.SlackConfig
	rn.PagerDutyConfig = *n.PagerDutyConfig

	rn.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("notifier", rn.Id)
	rn.Actions["validate"] = apiContext.UrlBuilder.ReferenceLink(rn.Resource) + "?action=validate"

	return rn
}

func toNotifierCRD(rn *Notifier, env string) *v1beta1.Notifier {
	n := &v1beta1.Notifier{
		EmailConfig:     &rn.EmailConfig,
		SlackConfig:     &rn.SlackConfig,
		PagerDutyConfig: &rn.PagerDutyConfig,
	}

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

	rn.EmailRecipient = *n.EmailRecipient
	rn.SlackRecipient = *n.SlackRecipient
	rn.PagerDutyRecipient = *n.PagerDutyRecipient

	rn.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("recipient", rn.Id)

	return rn
}

func toRecipientCRD(rn *Recipient, env string) *v1beta1.Recipient {
	n := &v1beta1.Recipient{
		ObjectMeta: metav1.ObjectMeta{
			Name: rn.Id,
		},
		EmailRecipient:     &rn.EmailRecipient,
		SlackRecipient:     &rn.SlackRecipient,
		PagerDutyRecipient: &rn.PagerDutyRecipient,
	}

	return n
}

func toAlertResource(apiContext *api.ApiContext, a *v1beta1.Alert) *Alert {
	ra := &Alert{
		Description:  a.Description,
		State:        "inactive",
		SendResolved: a.SendResolved,
		Severity:     a.Severity,
		ObjectType:   a.Object,
		ObjectID:     a.ObjectID,
		ServiceRule:  a.ServiceRule,
		RecipientID:  a.RecipientID,
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
		},
		Description:  ra.Description,
		SendResolved: ra.SendResolved,
		Severity:     ra.Severity,
		Object:       ra.ObjectType,
		ObjectID:     ra.ObjectID,
		ServiceRule:  ra.ServiceRule,
		RecipientID:  ra.RecipientID,
	}

	return alert
}
