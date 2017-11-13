package util

import (
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	v1beta1 "github.com/zionwu/alertmanager-operator/client/v1beta1"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateUUID() string {
	u1 := uuid.NewV4()
	return fmt.Sprintf("%s", u1)
}

func WaitForCRDReady(listFunc func(opts metav1.ListOptions) (runtime.Object, error)) error {
	err := wait.Poll(3*time.Second, 10*time.Minute, func() (bool, error) {
		_, err := listFunc(metav1.ListOptions{})
		if err != nil {
			if se, ok := err.(*apierrors.StatusError); ok {
				if se.Status().Code == http.StatusNotFound {
					return false, nil
				}
			}
			return false, err
		}
		return true, nil
	})

	return errors.Wrap(err, fmt.Sprintf("timed out waiting for Custom Resoruce"))
}

func NewAlertCustomResourceDefinition() *extensionsobj.CustomResourceDefinition {
	return &extensionsobj.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.AlertName + "." + v1beta1.Group,
		},
		Spec: extensionsobj.CustomResourceDefinitionSpec{
			Group:   v1beta1.Group,
			Version: v1beta1.Version,
			Scope:   extensionsobj.NamespaceScoped,
			Names: extensionsobj.CustomResourceDefinitionNames{
				Plural: v1beta1.AlertName,
				Kind:   v1beta1.AlertsKind,
			},
		},
	}
}

func NewNotifierCustomResourceDefinition() *extensionsobj.CustomResourceDefinition {
	return &extensionsobj.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.NotifierName + "." + v1beta1.Group,
		},
		Spec: extensionsobj.CustomResourceDefinitionSpec{
			Group:   v1beta1.Group,
			Version: v1beta1.Version,
			Scope:   extensionsobj.ClusterScoped,
			Names: extensionsobj.CustomResourceDefinitionNames{
				Plural: v1beta1.NotifierName,
				Kind:   v1beta1.NotifiersKind,
			},
		},
	}
}

func NewRecipientCustomResourceDefinition() *extensionsobj.CustomResourceDefinition {
	return &extensionsobj.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1.RecipientName + "." + v1beta1.Group,
		},
		Spec: extensionsobj.CustomResourceDefinitionSpec{
			Group:   v1beta1.Group,
			Version: v1beta1.Version,
			Scope:   extensionsobj.NamespaceScoped,
			Names: extensionsobj.CustomResourceDefinitionNames{
				Plural: v1beta1.RecipientName,
				Kind:   v1beta1.RecipientsKind,
			},
		},
	}
}
