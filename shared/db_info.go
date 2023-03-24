package shared

import (
	"database/sql"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

type DbConnector interface {
	Connect(connectInfo DbServerConnectInfo) (*sql.DB, error)
}

type DbActions interface {
	BuildContainer(scriptName string) v1.Container
	BuildBackupContainer() v1.Container
	BuildRestoreContainer() v1.Container
}

type DbActionsBase struct {
	Db       *dboperatorv1alpha1.Db
	DbServer *dboperatorv1alpha1.DbServer
	Options  map[string]string
}
