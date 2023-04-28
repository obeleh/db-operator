package controllers

import (
	"fmt"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type LazyDbServerHelper struct {
	*shared.K8sClient
	DbServerName string
	*dboperatorv1alpha1.DbServer
	dbServerCredentials *shared.Credentials
	userCredentials     map[string]*LazyUserHelper
}

func (h *LazyDbServerHelper) GetUser(userName string) (*LazyUserHelper, error) {
	lazyUserHelper, found := h.userCredentials[userName]
	if !found {
		lazyUserHelper := NewLazyUserHelper(h.K8sClient, userName)
		h.userCredentials[userName] = lazyUserHelper
	}
	return lazyUserHelper, nil
}

func NewLazyDbServerHelper(k8sClient *shared.K8sClient, dbServerName string) *LazyDbServerHelper {
	return &LazyDbServerHelper{
		K8sClient:    k8sClient,
		DbServerName: dbServerName,
	}
}

func (h *LazyDbServerHelper) GetDbServer() (*dboperatorv1alpha1.DbServer, error) {
	if h.DbServer == nil {
		dbServer, err := GetDbServer(h.DbServerName, h.Client, h.NsNm.Namespace)
		if err != nil {
			return nil, err
		}
		h.DbServer = dbServer
	}

	return h.DbServer, nil
}

func (h *LazyDbServerHelper) GetConnectInfo(databaseName *string) (*shared.DbServerConnectInfo, error) {
	dbServer, err := h.GetDbServer()
	if err != nil {
		return nil, err
	}

	credentials, err := h.GetCredentials()
	if err != nil {
		return nil, err
	}
	connectInfo := &shared.DbServerConnectInfo{
		Host:        dbServer.Spec.Address,
		Port:        dbServer.Spec.Port,
		Credentials: *credentials,
	}

	if len(dbServer.Spec.Options) > 0 {
		connectInfo.Options = dbServer.Spec.Options
	}

	if databaseName != nil {
		connectInfo.Database = *databaseName
	}

	return connectInfo, nil
}

func (h *LazyDbServerHelper) GetCredentials() (*shared.Credentials, error) {
	if h.dbServerCredentials == nil {
		dbServer, err := h.GetDbServer()
		if err != nil {
			return nil, err
		}

		secretName := types.NamespacedName{
			Name:      dbServer.Spec.SecretName,
			Namespace: dbServer.Namespace,
		}

		credentials := shared.Credentials{
			UserName:     dbServer.Spec.UserName,
			SourceSecret: &secretName,
		}

		dbServerSecret := v1.Secret{}

		err = h.Client.Get(h.Ctx, secretName, &dbServerSecret)
		if err != nil {
			err = fmt.Errorf("failed to get secret: %s %s", dbServer.Spec.SecretName, err)
			return nil, err
		}

		passwordBytes, found := dbServerSecret.Data[shared.Nvl(dbServer.Spec.PasswordKey, "password")]
		if found {
			password := string(passwordBytes)
			credentials.Password = &password
		}

		keys := []struct {
			specKey  *string
			credsKey **string
		}{
			{&dbServer.Spec.CaCertKey, &credentials.CaCert},
			{&dbServer.Spec.TlsKeyKey, &credentials.TlsKey},
			{&dbServer.Spec.TlsCrtKey, &credentials.TlsCrt},
		}

		for _, key := range keys {
			if *key.specKey != "" {
				valueBytes, found := dbServerSecret.Data[*key.specKey]
				if !found {
					return nil, fmt.Errorf("key '%s' not found in secret %s.%s", *key.specKey, dbServer.Namespace, dbServer.Spec.SecretName)
				}
				value := string(valueBytes)
				*key.credsKey = &value
			}
		}

		h.dbServerCredentials = &credentials
	}

	return h.dbServerCredentials, nil
}
