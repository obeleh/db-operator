package controllers

import (
	"fmt"
	"strings"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
	"k8s.io/apimachinery/pkg/types"
)

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
