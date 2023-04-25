package controllers

import (
	"fmt"
	"strings"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers"
	"github.com/obeleh/db-operator/dbservers/postgres"
	"github.com/obeleh/db-operator/shared"
	v1 "k8s.io/api/core/v1"
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

type LazyDbHelper struct {
	*shared.K8sClient
	DbName string
	*dboperatorv1alpha1.Db
	lazyDbServerHelper *LazyDbServerHelper
	conn               shared.DbServerConnectionInterface
	connLoaded         bool
	grantorNames       []string
}

func NewLazyDbHelper(k8sClient *shared.K8sClient, DbName string, grantorNames []string) *LazyDbHelper {
	return &LazyDbHelper{
		K8sClient:    k8sClient,
		DbName:       DbName,
		grantorNames: grantorNames,
	}
}

func (h *LazyDbHelper) GetDb() (*dboperatorv1alpha1.Db, error) {
	if h.Db == nil {
		db := &dboperatorv1alpha1.Db{}
		nsName := types.NamespacedName{
			Name:      h.DbName,
			Namespace: h.NsNm.Namespace,
		}
		err := h.Client.Get(h.Ctx, nsName, db)
		if err != nil {
			return nil, err
		}
		h.Db = db
	}
	return h.Db, nil
}

func (h *LazyDbHelper) GetDbServer() (*dboperatorv1alpha1.DbServer, error) {
	if h.lazyDbServerHelper == nil {
		db, err := h.GetDb()
		if err != nil {
			return nil, err
		}

		h.lazyDbServerHelper = NewLazyDbServerHelper(h.K8sClient, db.Spec.Server)
	}

	return h.lazyDbServerHelper.GetDbServer()
}

func (h *LazyDbHelper) GetDbConnection() (shared.DbServerConnectionInterface, error) {
	if !h.connLoaded {
		dbServer, err := h.GetDbServer()
		if err != nil {
			return nil, err
		}
		if h.lazyDbServerHelper == nil {
			return nil, fmt.Errorf("Expected lazyDbServerHelper to be loaded")
		}

		connectInfo, err := h.lazyDbServerHelper.GetConnectInfo(&h.DbName)
		if err != nil {
			return nil, err
		}

		userCredentials := map[string]*shared.Credentials{}
		if len(h.grantorNames) > 0 {
			for _, userName := range h.grantorNames {
				lazyUserHelper, err := h.lazyDbServerHelper.GetUser(userName)
				if err != nil {
					return nil, err
				}
				credentials, err := lazyUserHelper.GetCredentials()
				if err != nil {
					return nil, err
				}
				userCredentials[userName] = credentials
			}
		}

		return dbservers.GetServerConnection(dbServer.Spec.ServerType, connectInfo, userCredentials)

	}
	return h.conn, nil
}

func (h *LazyDbHelper) GetPgConnection() (*postgres.PostgresConnection, error) {
	conn, err := h.GetDbConnection()
	if err != nil {
		return nil, err
	}

	pgConn := conn.(*postgres.PostgresConnection)
	if conn == nil {
		return nil, fmt.Errorf("Backing database is not postgres connection compatible")
	}
	return pgConn, nil
}

func (h *LazyDbHelper) CleanupConn() {
	if h.connLoaded {
		h.conn.Close()
	}
}

type LazyBackupTargetHelper struct {
	*shared.K8sClient
	BackupTargetName string
	*dboperatorv1alpha1.BackupTarget
	lazyDbHelper             *LazyDbHelper
	lazyStorageActionsHelper *LazyStorageActionsHelper
}

func NewLazyBackupTargetHelper(k8sClient *shared.K8sClient, backupTargetName string) *LazyBackupTargetHelper {
	return &LazyBackupTargetHelper{
		K8sClient:        k8sClient,
		BackupTargetName: backupTargetName,
	}
}

func (h *LazyBackupTargetHelper) GetBackupTarget() (*dboperatorv1alpha1.BackupTarget, error) {
	if h.BackupTarget == nil {
		backupTargetCr := &dboperatorv1alpha1.BackupTarget{}
		nsName := types.NamespacedName{
			Name:      h.BackupTargetName,
			Namespace: h.NsNm.Namespace,
		}
		err := h.Client.Get(h.Ctx, nsName, backupTargetCr)
		if err != nil {
			return nil, err
		}
		h.BackupTarget = backupTargetCr
	}
	return h.BackupTarget, nil
}

func (h *LazyBackupTargetHelper) GetStorageActions() (StorageActions, error) {
	if h.lazyStorageActionsHelper == nil {
		backupTarget, err := h.GetBackupTarget()
		if err != nil {
			return nil, err
		}
		h.lazyStorageActionsHelper = NewLazyStorageActionsHelper(h.K8sClient, backupTarget.Spec.StorageType, backupTarget.Spec.StorageLocation)
	}
	return h.lazyStorageActionsHelper.GetStorageActions()
}

func (h *LazyBackupTargetHelper) GetPgConnection() (*postgres.PostgresConnection, error) {
	if h.lazyDbHelper == nil {
		backupTarget, err := h.GetBackupTarget()
		if err != nil {
			return nil, err
		}
		h.lazyDbHelper = NewLazyDbHelper(h.K8sClient, backupTarget.Spec.DbName, nil)
	}
	return h.lazyDbHelper.GetPgConnection()
}

func (h *LazyBackupTargetHelper) GetDbName() (string, error) {
	backupTarget, err := h.GetBackupTarget()
	if err != nil {
		return "", err
	}

	return backupTarget.Spec.DbName, nil
}

func (h *LazyBackupTargetHelper) GetBucketStorageInfo() (shared.BucketStorageInfo, error) {
	storageActions, err := h.GetStorageActions()
	if err != nil {
		return shared.BucketStorageInfo{}, err
	}

	return storageActions.GetBucketStorageInfo(*h.K8sClient)
}

func (h *LazyBackupTargetHelper) CleanupConn() {
	if h.lazyDbHelper != nil {
		h.lazyDbHelper.CleanupConn()
	}
}

type LazyStorageActionsHelper struct {
	*shared.K8sClient
	StorageType          string
	StorageLocation      string
	storageActions       StorageActions
	storageActionsLoaded bool
}

func NewLazyStorageActionsHelper(k8sClient *shared.K8sClient, storageType string, storageLocation string) *LazyStorageActionsHelper {
	return &LazyStorageActionsHelper{
		K8sClient:       k8sClient,
		StorageType:     storageType,
		StorageLocation: storageLocation,
	}
}

func (h *LazyStorageActionsHelper) GetStorageActions() (StorageActions, error) {
	if !h.storageActionsLoaded {
		if strings.ToLower(h.StorageType) == "s3" {
			s3Storage := &dboperatorv1alpha1.S3Storage{}
			nsName := types.NamespacedName{
				Name:      h.StorageLocation,
				Namespace: h.NsNm.Namespace,
			}

			err := h.Client.Get(h.Ctx, nsName, s3Storage)
			if err != nil {
				return nil, err
			}
			s3StorageInfo := &S3StorageInfo{
				S3Storage: *s3Storage,
			}
			h.storageActions = s3StorageInfo
			h.storageActionsLoaded = true
		} else {
			return nil, fmt.Errorf("unknown storage type %s", h.StorageType)
		}
	}
	return h.storageActions, nil
}
