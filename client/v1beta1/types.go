package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Notifier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NotifierSpec `json:"spec"`
	//TODO: check what should do with status
	Status *NotifierStatus `json:"status,omitempty"`
}

type NotifierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []*Notifier `json:"items"`
}

type NotifierSpec struct {
	PodMetadata     *metav1.ObjectMeta   `json:"podMetadata,omitempty"`
	Kind            string               `json:"kind,omitempty"`
	EmailConfig     *EmailConfigSpec     `json:"email_config,omitempty"`
	SlackConfig     *SlackConfigSpec     `json:"slack_config,omitempty"`
	PagerDutyConfig *PagerDutyConfigSpec `json:"pagerduty_config,omitempty"`
}

type PagerDutyConfigSpec struct {
	PagerDutyUrl string `json:"pagerduty_url"`
}

type SlackConfigSpec struct {
	SlackApiUrl string `json:"slack_api_url"`
}

type EmailConfigSpec struct {
	SMTPFrom         string `json:"smtp_from"`
	SMTPSmartHost    string `json:"smtp_smarthost"`
	SMTPAuthUserName string `json:"smtp_auth_username"`
	SMTPAuthPassword string `json:"smtp_auth_password"`
	SMTPAuthSecret   string `json:"smtp_auth_secret"`
	SMTPAuthIdentity string `json:"smtp_auth_identity"`
	SMTPRequireTLS   string `json:"smtp_required_tls"`
}

type NotifierStatus struct {
	Paused bool `json:"paused"`
}

type Recipient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RecipientSpec `json:"spec"`

	//TODO: check what should do with status
	Status *RecipientStatus `json:"status,omitempty"`
}

type RecipientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []*Recipient `json:"items"`
}

type RecipientSpec struct {
	PodMetadata        *metav1.ObjectMeta      `json:"podMetadata,omitempty"`
	Kind               string                  `json:"kind,omitempty"`
	EmailRecipient     *EmailRecipientSpec     `json:"email_recipient,omitempty"`
	SlackRecipient     *SlackRecipientSpec     `json:"slack_recipient,omitempty"`
	PagerDutyRecipient *PagerDutyRecipientSpec `json:"pagerduty_recipient,omitempty"`
}

type PagerDutyRecipientSpec struct {
	ServiceKey string `json:"service_key"`
}

type SlackRecipientSpec struct {
	Channel string `json:"channel"`
}

type EmailRecipientSpec struct {
	Address string `json:"address"`
}

type RecipientStatus struct {
	Paused bool `json:"paused"`
}

type Alert struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AlertSpec `json:"spec"`
	//TODO: check what should do with status
	Status *AlertStatus `json:"status,omitempty"`
}

type AlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []*Alert `json:"items"`
}

type AlertSpec struct {
	PodMetadata *metav1.ObjectMeta `json:"podMetadata,omitempty"`

	Name         string          `json:"name,omitempty"`
	Status       string          `json:"status,omitempty"`
	SendResolved bool            `json:"send_resolved,omitempty"`
	Severity     string          `json:"severity, omitempty"`
	Object       string          `json:"object, omitempty"`
	ObjectID     string          `json:"object_id, omitempty"`
	ServiceRule  ServiceRuleSpec `json:"service_rule, omitempty"`
	RecipientID  string          `json:"recipient_id, omitempty"`
}

type ServiceRuleSpec struct {
	UnhealthyPercetage string `json:"unhealthy_percetage, omitempty"`
}

type AlertStatus struct {
	Paused bool `json:"paused"`
}
