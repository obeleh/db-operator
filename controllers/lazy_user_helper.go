// Would have been nice if this file did what its name implies
package controllers

import (
	"fmt"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	"k8s.io/apimachinery/pkg/types"
)

type LazyUserHelper struct {
	*shared.K8sClient
	UserName    string
	user        *dboperatorv1alpha1.User
	credentials *shared.Credentials
}

func NewLazyUserHelper(k8sClient *shared.K8sClient, userName string) *LazyUserHelper {
	return &LazyUserHelper{
		K8sClient: k8sClient,
		UserName:  userName,
	}
}

func (h *LazyUserHelper) GetCredentials() (*shared.Credentials, error) {
	if h.credentials == nil {
		nsm := types.NamespacedName{
			Name:      h.UserName,
			Namespace: h.NsNm.Namespace,
		}
		user := &dboperatorv1alpha1.User{}

		err := h.Client.Get(h.Ctx, nsm, user)
		if err != nil {
			h.Log.Info(fmt.Sprintf("%T: %s does not exist", user, h.UserName))
			return nil, err
		}
		credentials, err := GetUserCredentials(user, h.K8sClient.Client, h.K8sClient.Ctx)
		if err != nil {
			return nil, err
		}

		h.user = user
		h.credentials = credentials
	}
	return h.credentials, nil
}
