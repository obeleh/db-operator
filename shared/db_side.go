package shared

import (
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
)

type DbSideUser struct {
	UserName   string
	Attributes string
}

type DbSideDb struct {
	DatbaseName string
}

type DbServerConnectInfo struct {
	Host string
	Port int
	Credentials
	Database string
	Options  map[string]string
}

type DbServerConnectionInterface interface {
	GetConnectionString() string
	CreateUser(userName string, password string) error
	DropUser(userName string) error
	GetUsers() (map[string]DbSideUser, error)
	CreateDb(dbName string) error
	DropDb(dbName string) error
	GetDbs() (map[string]DbSideDb, error)
	UpdateUserPrivs(string, string, []dboperatorv1alpha1.DbPriv) (bool, error)
	ScopeToDbName(scope string) (string, error)
	Close() error
}
