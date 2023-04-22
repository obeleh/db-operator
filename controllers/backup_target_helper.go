package controllers

import (
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
)

type BackupInfo struct {
	*dboperatorv1alpha1.BackupTarget
	StorageActions
	*dboperatorv1alpha1.Db
	*dboperatorv1alpha1.DbServer
	shared.DbServerConnectionInterface
	shared.BucketStorageInfo
}

type BackupTargetHelper struct {
	Reco
}

func (b *BackupTargetHelper) GetBackupInfo(backupTargetName string) (BackupInfo, error) {
	backupTarget, err := b.GetBackupTarget(backupTargetName)
	if err != nil {
		return BackupInfo{}, err
	}
	actions, err := b.GetStorageActions(backupTarget.Spec.StorageType, backupTarget.Spec.StorageLocation)
	if err != nil {
		return BackupInfo{}, err
	}
	db, dbServer, err := b.GetDbServerFromDbName(backupTarget.Spec.DbName)
	if err != nil {
		return BackupInfo{}, err
	}

	conn, err := b.GetDbConnection(dbServer, nil, &backupTarget.Spec.DbName)

	storeageInfo, err := actions.GetBucketStorageInfo(b.K8sClient)
	if err != nil {
		return BackupInfo{}, err
	}

	return BackupInfo{
		backupTarget,
		actions,
		db,
		dbServer,
		conn,
		storeageInfo,
	}, nil
}
