package api

import (
	"net/http"
	"time"

	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	mclient "github.com/zionwu/alertmanager-operator/client"
	"github.com/zionwu/alertmanager-operator/client/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Config struct {
	ManagerUrl    string
	SecretName    string
	Namespace     string
	ConfigMapName string
	PrometheusURL string
}

type Server struct {
	clientset kubernetes.Interface
	mclient   mclient.Interface
	cfg       *Config
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
	ResolveTimeout string                  `json:"resolveTimeout"`
	SlackConfig    v1beta1.SlackConfigSpec `json:"slackConfig"`
	EmailConfig    v1beta1.EmailConfigSpec `json:"emailConfig"`
	//PagerDutyConfig v1beta1.PagerDutyConfigSpec `json:"pagerdutyConfig"`
}

type Alert struct {
	client.Resource
	Description     string                      `json:"description"`
	State           string                      `json:"state"`
	Severity        string                      `json:"severity"`
	TargetType      string                      `json:"targetType"`
	TargetID        string                      `json:"targetId"`
	NodeRule        v1beta1.NodeRuleSpec        `json:"nodeRule"`
	DeploymentRule  v1beta1.RuleSpec            `json:"deploymentRule"`
	StatefulsetRule v1beta1.RuleSpec            `json:"statefulsetRule"`
	DaemonsetRule   v1beta1.RuleSpec            `json:"daemonsetRule"`
	AdvancedOptions v1beta1.AdvancedOptionsSpec `json:"advancedOptions"`
	MetricRule      v1beta1.MetricRuleSpec      `json:"metricRule"`
	Namespace       string                      `json:"namespace"`
	RecipientID     string                      `json:"recipientId"`
	StartsAt        time.Time                   `json:"startsAt,omitempty"`
	EndsAt          time.Time                   `json:"endsAt,omitempty"`
}

type Recipient struct {
	client.Resource
	Namespace          string                         `json:"namespace"`
	RecipientType      string                         `json:"recipientType"`
	SlackRecipient     v1beta1.SlackRecipientSpec     `json:"slackRecipient"`
	EmailRecipient     v1beta1.EmailRecipientSpec     `json:"emailRecipient"`
	PagerDutyRecipient v1beta1.PagerDutyRecipientSpec `json:"pagerdutyRecipient"`
	WebhookRecipient   v1beta1.WebhookRecipientSpec   `json:"webhookRecipient"`
}

func NewServer(config *rest.Config, cfg *Config) *Server {

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	mclient, err := mclient.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return &Server{
		clientset: clientset,
		mclient:   mclient,
		cfg:       cfg,
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

	//for k8s resource
	podSchema(schemas.AddType("pod", Pod{}))
	nodeSchema(schemas.AddType("node", Node{}))
	namespaceSchema(schemas.AddType("namespace", Namespace{}))
	deploymentSchema(schemas.AddType("deployment", Deployment{}))
	daemonsetSchema(schemas.AddType("daemonset", DaemonSet{}))
	statefulsetSchema(schemas.AddType("statefulset", StatefulSet{}))

	return schemas
}

func alertSchema(alert *client.Schema) {

	alert.CollectionMethods = []string{http.MethodGet, http.MethodPost}
	alert.ResourceMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut}

	severity := alert.ResourceFields["severity"]
	severity.Create = true
	severity.Required = true
	severity.Type = "enum"
	severity.Options = []string{"info", "warning", "critical"}
	severity.Default = "critical"
	alert.ResourceFields["severity"] = severity

	state := alert.ResourceFields["state"]
	state.Create = false
	state.Update = false
	state.Type = "enum"
	state.Default = "enabled"
	state.Options = []string{"enabled", "disabled", "active", "suppressed"}
	alert.ResourceFields["state"] = state

	description := alert.ResourceFields["description"]
	description.Create = true
	description.Update = false
	alert.ResourceFields["description"] = description

	targetType := alert.ResourceFields["targetType"]
	targetType.Create = true
	targetType.Update = false
	targetType.Type = "enum"
	targetType.Options = []string{"pod", "node", "deployment", "daemonset", "statefulset", "metric"}
	alert.ResourceFields["targetType"] = targetType

	targetId := alert.ResourceFields["targetId"]
	targetId.Create = true
	targetId.Update = false
	alert.ResourceFields["targetId"] = targetId

	recipientId := alert.ResourceFields["recipientId"]
	recipientId.Create = true
	recipientId.Update = true
	recipientId.Type = "reference[recipient]"
	alert.ResourceFields["recipientId"] = recipientId

	namespace := alert.ResourceFields["namespace"]
	namespace.Create = true
	namespace.Required = true
	namespace.Update = false
	alert.ResourceFields["namespace"] = namespace

	alert.ResourceActions = map[string]client.Action{
		"silence": {
			Output: "alert",
		},
		"unsilence": {
			Output: "alert",
		},
		"enable": {
			Output: "alert",
		},
		"disable": {
			Output: "alert",
		},
	}

}

func recipientSchema(recipient *client.Schema) {
	recipient.CollectionMethods = []string{http.MethodGet, http.MethodPost}
	recipient.ResourceMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut}

	namespace := recipient.ResourceFields["namespace"]
	namespace.Create = true
	namespace.Required = true
	namespace.Update = false
	recipient.ResourceFields["namespace"] = namespace

	recipientType := recipient.ResourceFields["recipientType"]
	recipientType.Create = true
	recipientType.Update = false
	recipientType.Type = "enum"
	recipientType.Options = []string{"email", "slack", "pagerduty", "webhook"}
	recipient.ResourceFields["recipientType"] = recipientType
}

func notifierSchema(notifier *client.Schema) {
	notifier.CollectionMethods = []string{http.MethodGet}
	notifier.ResourceMethods = []string{http.MethodGet, http.MethodPut}

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
	rn.ResolveTimeout = n.ResolveTimeout
	//rn.PagerDutyConfig = *n.PagerDutyConfig

	rn.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("notifier", rn.Id)

	return rn
}

func toNotifierCRD(rn *Notifier) *v1beta1.Notifier {
	n := &v1beta1.Notifier{
		ObjectMeta: metav1.ObjectMeta{
			Name: rn.Id,
		},
		EmailConfig:    &rn.EmailConfig,
		SlackConfig:    &rn.SlackConfig,
		ResolveTimeout: rn.ResolveTimeout,
		//PagerDutyConfig: &rn.PagerDutyConfig,
	}

	n.EmailConfig.SMTPAuthPassword = "<secret>"
	n.SlackConfig.SlackApiUrl = "<secret>"

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

	rn.Namespace = n.Namespace
	rn.RecipientType = n.RecipientType
	rn.EmailRecipient = *n.EmailRecipient
	rn.SlackRecipient = *n.SlackRecipient
	rn.PagerDutyRecipient = *n.PagerDutyRecipient
	rn.WebhookRecipient = *n.WebhookRecipient

	rn.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("recipient", rn.Id)

	return rn
}

func toRecipientCRD(rn *Recipient) *v1beta1.Recipient {
	n := &v1beta1.Recipient{
		ObjectMeta: metav1.ObjectMeta{
			Name: rn.Id,
		},
		RecipientType:      rn.RecipientType,
		EmailRecipient:     &rn.EmailRecipient,
		SlackRecipient:     &rn.SlackRecipient,
		PagerDutyRecipient: &rn.PagerDutyRecipient,
		WebhookRecipient:   &rn.WebhookRecipient,
	}

	n.PagerDutyRecipient.ServiceKey = "<secret>"

	return n
}

func toAlertResource(apiContext *api.ApiContext, a *v1beta1.Alert) *Alert {
	ra := &Alert{
		Namespace:       a.Namespace,
		Description:     a.Description,
		State:           a.State,
		Severity:        a.Severity,
		TargetType:      a.TargetType,
		TargetID:        a.TargetID,
		RecipientID:     a.RecipientID,
		NodeRule:        *a.NodeRule,
		DeploymentRule:  *a.DeploymentRule,
		StatefulsetRule: *a.StatefulsetRule,
		DaemonsetRule:   *a.DaemonsetRule,
		MetricRule:      *a.MetricRule,
		AdvancedOptions: *a.AdvancedOptions,
		StartsAt:        a.StartsAt,
		EndsAt:          a.EndsAt,
	}

	ra.Resource = client.Resource{
		//TODO: decide what should be id
		Id:      a.Name,
		Type:    "alert",
		Actions: map[string]string{},
		Links:   map[string]string{},
	}

	ra.Resource.Links["self"] = apiContext.UrlBuilder.ReferenceByIdLink("alert", ra.Id) + "?namespace=" + a.Namespace
	ra.Resource.Links["update"] = apiContext.UrlBuilder.ReferenceByIdLink("alert", ra.Id)
	ra.Resource.Links["remove"] = apiContext.UrlBuilder.ReferenceByIdLink("alert", ra.Id) + "?namespace=" + a.Namespace
	ra.Resource.Links["recipient"] = apiContext.UrlBuilder.ReferenceByIdLink("recipient", ra.RecipientID) + "?namespace=" + a.Namespace
	ra.Actions["enable"] = apiContext.UrlBuilder.ReferenceLink(ra.Resource) + "?action=enable&" + "namespace=" + a.Namespace
	ra.Actions["disable"] = apiContext.UrlBuilder.ReferenceLink(ra.Resource) + "?action=disable&" + "namespace=" + a.Namespace
	ra.Actions["silence"] = apiContext.UrlBuilder.ReferenceLink(ra.Resource) + "?action=silence&" + "namespace=" + a.Namespace
	ra.Actions["unsilence"] = apiContext.UrlBuilder.ReferenceLink(ra.Resource) + "?action=unsilence&" + "namespace=" + a.Namespace

	return ra
}

func toAlertCRD(ra *Alert) *v1beta1.Alert {
	alert := &v1beta1.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name: ra.Id,
		},
		Description:     ra.Description,
		Severity:        ra.Severity,
		TargetType:      ra.TargetType,
		TargetID:        ra.TargetID,
		RecipientID:     ra.RecipientID,
		NodeRule:        &ra.NodeRule,
		DeploymentRule:  &ra.DeploymentRule,
		StatefulsetRule: &ra.StatefulsetRule,
		DaemonsetRule:   &ra.DaemonsetRule,
		MetricRule:      &ra.MetricRule,
		AdvancedOptions: &ra.AdvancedOptions,
		State:           ra.State,
		StartsAt:        ra.StartsAt,
		EndsAt:          ra.EndsAt,
	}

	return alert
}
