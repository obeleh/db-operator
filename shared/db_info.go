package shared

import (
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

type DbActions interface {
	GetDbConnection() (DbServerConnectionInterface, error)
	BuildContainer(scriptName string) v1.Container
	BuildBackupContainer() v1.Container
	BuildRestoreContainer() v1.Container
}

type DbInfo struct {
	Db       *dboperatorv1alpha1.Db
	DbServer *dboperatorv1alpha1.DbServer
	Credentials
	Options map[string]string
}
