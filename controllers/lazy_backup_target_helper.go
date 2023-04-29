package controllers

import (
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers"
	"github.com/obeleh/db-operator/dbservers/postgres"
	"github.com/obeleh/db-operator/shared"
	"k8s.io/apimachinery/pkg/types"
)

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

func (h *LazyBackupTargetHelper) GetLazyDbHelper() (*LazyDbHelper, error) {
	if h.lazyDbHelper == nil {
		backupTarget, err := h.GetBackupTarget()
		if err != nil {
			return nil, err
		}
		h.lazyDbHelper = NewLazyDbHelper(h.K8sClient, backupTarget.Spec.DbName, nil)
	}
	return h.lazyDbHelper, nil
}

func (h *LazyBackupTargetHelper) GetPgConnection() (*postgres.PostgresConnection, error) {
	lazyDbHelper, err := h.GetLazyDbHelper()
	if err != nil {
		return nil, err
	}
	return lazyDbHelper.GetPgConnection()
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

func (h *LazyBackupTargetHelper) GetServerActions() (shared.DbActions, error) {
	lazyDbHelper, err := h.GetLazyDbHelper()
	if err != nil {
		return nil, err
	}

	db, err := lazyDbHelper.GetDb()
	if err != nil {
		return nil, err
	}

	dbServer, err := lazyDbHelper.GetDbServer()
	if err != nil {
		return nil, err
	}

	return dbservers.GetServerActions(dbServer, db, dbServer.Spec.Options)
}
