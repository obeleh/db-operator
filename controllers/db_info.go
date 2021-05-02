package controllers

import (
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

type DbActions interface {
	GetDbConnection(dbInfo *DbInfo) (DbServerConnectionInterface, error)
	buildContainer(dbInfo *DbInfo, scriptName string) v1.Container
	BuildBackupContainer(dbInfo *DbInfo) v1.Container
	BuildRestoreContainer(dbInfo *DbInfo) v1.Container
}

type DbInfo struct {
	Db       *dboperatorv1alpha1.Db
	DbServer *dboperatorv1alpha1.DbServer
	Password string
	Actions  DbActions
}

func (d *DbInfo) GetDbConnection() (DbServerConnectionInterface, error) {
	return d.Actions.GetDbConnection(d)
}

func (d *DbInfo) BuildBackupContainer() v1.Container {
	return d.Actions.BuildBackupContainer(d)
}

func (d *DbInfo) BuildRestoreContainer() v1.Container {
	return d.Actions.BuildRestoreContainer(d)
}
