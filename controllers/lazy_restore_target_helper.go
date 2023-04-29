package controllers

import (
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	"k8s.io/apimachinery/pkg/types"
)

type LazyRestoreTargetHelper struct {
	LazyTargetHelperBase
	RestoreTargetName string
	*dboperatorv1alpha1.RestoreTarget
}

func NewLazyRestoreTargetHelper(k8sClient *shared.K8sClient, restoreTargetName string) *LazyRestoreTargetHelper {
	var h LazyRestoreTargetHelper

	h = LazyRestoreTargetHelper{
		LazyTargetHelperBase: LazyTargetHelperBase{
			K8sClient: k8sClient,
			GetStorageTypeAndLocation: func() (string, string, error) {
				restoreTarget, err := h.GetRestoreTarget()
				if err != nil {
					return "", "", err
				}
				return restoreTarget.Spec.StorageType, restoreTarget.Spec.StorageLocation, nil
			},
			BuildLazyDbHelper: func() (*LazyDbHelper, error) {
				restoreTarget, err := h.GetRestoreTarget()
				if err != nil {
					return nil, err
				}
				return NewLazyDbHelper(h.K8sClient, restoreTarget.Spec.DbName, nil), nil
			},
		},
		RestoreTargetName: restoreTargetName,
	}

	return &h
}

func (h *LazyRestoreTargetHelper) GetRestoreTarget() (*dboperatorv1alpha1.RestoreTarget, error) {
	if h.RestoreTarget == nil {
		restoreTargetCr := &dboperatorv1alpha1.RestoreTarget{}
		nsName := types.NamespacedName{
			Name:      h.RestoreTargetName,
			Namespace: h.NsNm.Namespace,
		}
		err := h.Client.Get(h.Ctx, nsName, restoreTargetCr)
		if err != nil {
			return nil, err
		}
		h.RestoreTarget = restoreTargetCr
	}
	return h.RestoreTarget, nil
}

func (h *LazyRestoreTargetHelper) GetDbName() (string, error) {
	restoreTarget, err := h.GetRestoreTarget()
	if err != nil {
		return "", err
	}
	return restoreTarget.Spec.DbName, nil
}
