package controllers

import (
	"fmt"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers"
	"github.com/obeleh/db-operator/dbservers/postgres"
	"github.com/obeleh/db-operator/shared"
	"k8s.io/apimachinery/pkg/types"
)

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
			return nil, fmt.Errorf("expected lazyDbServerHelper to be loaded %w", err)
		}

		connectInfo, err := h.lazyDbServerHelper.GetConnectInfo(&h.DbName)
		if err != nil {
			return nil, err
		}

		userCredentials := map[string]*shared.Credentials{}
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
		return nil, fmt.Errorf("backing database is not postgres connection compatible: %w", err)
	}
	return pgConn, nil
}

func (h *LazyDbHelper) CleanupConn() {
	if h.connLoaded {
		h.conn.Close()
	}
}
