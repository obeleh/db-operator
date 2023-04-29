package controllers

import (
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	"k8s.io/apimachinery/pkg/types"
)

type LazyBackupTargetHelper struct {
	LazyTargetHelperBase
	BackupTargetName string
	*dboperatorv1alpha1.BackupTarget
}

func NewLazyBackupTargetHelper(k8sClient *shared.K8sClient, backupTargetName string) *LazyBackupTargetHelper {
	var h LazyBackupTargetHelper

	h = LazyBackupTargetHelper{
		LazyTargetHelperBase: LazyTargetHelperBase{
			K8sClient: k8sClient,
			GetStorageTypeAndLocation: func() (string, string, error) {
				backupTarget, err := h.GetBackupTarget()
				if err != nil {
					return "", "", err
				}
				return backupTarget.Spec.StorageType, backupTarget.Spec.StorageLocation, nil
			},
			BuildLazyDbHelper: func() (*LazyDbHelper, error) {
				backupTarget, err := h.GetBackupTarget()
				if err != nil {
					return nil, err
				}
				return NewLazyDbHelper(h.K8sClient, backupTarget.Spec.DbName, nil), nil
			},
		},
		BackupTargetName: backupTargetName,
	}

	return &h
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

func (h *LazyBackupTargetHelper) GetDbName() (string, error) {
	backupTarget, err := h.GetBackupTarget()
	if err != nil {
		return "", err
	}

	return backupTarget.Spec.DbName, nil
}
