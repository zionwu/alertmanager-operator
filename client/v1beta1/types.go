package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Notifier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	EmailConfig     *EmailConfigSpec     `json:"emailConfig,omitempty"`
	SlackConfig     *SlackConfigSpec     `json:"slackConfig,omitempty"`
	PagerDutyConfig *PagerDutyConfigSpec `json:"pagerdutyConfig,omitempty"`
	//TODO: check what should do with status
	Status *NotifierStatus `json:"status,omitempty"`
}

type NotifierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []*Notifier `json:"items"`
}

type PagerDutyConfigSpec struct {
	PagerDutyUrl string `json:"pagerdutyUrl"`
}

type SlackConfigSpec struct {
	SlackApiUrl string `json:"slackApiUrl"`
}

type EmailConfigSpec struct {
	SMTPFrom         string `json:"smtpFrom"`
	SMTPSmartHost    string `json:"smtpSmartHost"`
	SMTPAuthUserName string `json:"smtpAuthUsername"`
	SMTPAuthPassword string `json:"smtpAuthPassword"`
	SMTPAuthSecret   string `json:"smtpAuthSecret"`
	SMTPAuthIdentity string `json:"smtpAuthIdentity"`
	SMTPRequireTLS   bool   `json:"smtpRequiredTls"`
}

//TODO: decide the field in status
type NotifierStatus struct {
	Paused bool `json:"paused"`
}

type Recipient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	EmailRecipient     *EmailRecipientSpec     `json:"emailRecipient,omitempty"`
	SlackRecipient     *SlackRecipientSpec     `json:"slackRecipient,omitempty"`
	PagerDutyRecipient *PagerDutyRecipientSpec `json:"pagerdutyRecipient,omitempty"`

	//TODO: check what should do with status
	Status *RecipientStatus `json:"status,omitempty"`
}

type RecipientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []*Recipient `json:"items"`
}

type PagerDutyRecipientSpec struct {
	ServiceKey string `json:"serviceKey"`
}

type SlackRecipientSpec struct {
	Channel string `json:"channel"`
}

type EmailRecipientSpec struct {
	Address string `json:"address"`
}

//TODO: decide the field in status
type RecipientStatus struct {
	Paused bool `json:"paused"`
}

type Alert struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description  string          `json:"description,omitempty"`
	SendResolved bool            `json:"sendResolved,omitempty"`
	Severity     string          `json:"severity, omitempty"`
	Object       string          `json:"object, omitempty"`
	ObjectID     string          `json:"objectId, omitempty"`
	ServiceRule  ServiceRuleSpec `json:"serviceRule, omitempty"`
	RecipientID  string          `json:"recipientId, omitempty"`
	//TODO: check what should do with status
	Status *AlertStatus `json:"status,omitempty"`
}

type AlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []*Alert `json:"items"`
}

type ServiceRuleSpec struct {
	UnhealthyPercetage string `json:"unhealthyPercentage, omitempty"`
}

//TODO: decide the field in status
type AlertStatus struct {
	Paused bool `json:"paused"`
}
