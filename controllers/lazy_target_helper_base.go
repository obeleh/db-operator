package controllers

import (
	"github.com/obeleh/db-operator/dbservers"
	"github.com/obeleh/db-operator/dbservers/postgres"
	"github.com/obeleh/db-operator/shared"
)

type LazyTargetHelperBase struct {
	*shared.K8sClient
	lazyDbHelper              *LazyDbHelper
	lazyStorageActionsHelper  *LazyStorageActionsHelper
	GetStorageTypeAndLocation func() (string, string, error)
	BuildLazyDbHelper         func() (*LazyDbHelper, error)
}

func NewLazyTargetHelperBase(k8sClient *shared.K8sClient) *LazyTargetHelperBase {
	return &LazyTargetHelperBase{
		K8sClient: k8sClient,
	}
}

func (h *LazyTargetHelperBase) GetStorageActions() (StorageActions, error) {
	if h.lazyStorageActionsHelper == nil {
		storageType, storageLocation, err := h.GetStorageTypeAndLocation()
		if err != nil {
			return nil, err
		}
		h.lazyStorageActionsHelper = NewLazyStorageActionsHelper(h.K8sClient, storageType, storageLocation)
	}
	return h.lazyStorageActionsHelper.GetStorageActions()
}

func (h *LazyTargetHelperBase) GetLazyDbHelper() (*LazyDbHelper, error) {
	if h.lazyDbHelper == nil {
		helper, err := h.BuildLazyDbHelper()
		if err != nil {
			return nil, err
		}
		h.lazyDbHelper = helper
	}
	return h.lazyDbHelper, nil
}

func (h *LazyTargetHelperBase) GetPgConnection() (*postgres.PostgresConnection, error) {
	lazyDbHelper, err := h.GetLazyDbHelper()
	if err != nil {
		return nil, err
	}
	return lazyDbHelper.GetPgConnection()
}

func (h *LazyTargetHelperBase) GetBucketStorageInfo() (shared.BucketStorageInfo, error) {
	storageActions, err := h.GetStorageActions()
	if err != nil {
		return shared.BucketStorageInfo{}, err
	}

	return storageActions.GetBucketStorageInfo(*h.K8sClient)
}

func (h *LazyTargetHelperBase) CleanupConn() {
	if h.lazyDbHelper != nil {
		h.lazyDbHelper.CleanupConn()
	}
}

func (h *LazyTargetHelperBase) GetServerActions() (shared.DbActions, error) {
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

func (h *LazyTargetHelperBase) GetStorageInfoAndActions() (StorageActions, shared.DbActions, error) {
	storageActions, err := h.GetStorageActions()
	if err != nil {
		return nil, nil, err
	}

	serverActions, err := h.GetServerActions()
	if err != nil {
		return nil, nil, err
	}

	return storageActions, serverActions, nil
}
