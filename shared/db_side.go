package shared

import (
	"database/sql"
	"fmt"

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
	Host     string
	Port     int
	UserName string
	Password string
	Database string
	Options  map[string]string
}

type DbServerConnection struct {
	DbServerConnectInfo
	Conn   *sql.DB
	Driver string
	DbServerConnectionInterface
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
	Close() error
}

func (s *DbServerConnection) GetDbConnection() (*sql.DB, error) {
	if s.Conn == nil {
		var err error
		connStr := s.GetConnectionString()

		s.Conn, err = sql.Open(s.Driver, connStr)
		if err != nil {
			return nil, fmt.Errorf("failed to open a %s DB connection to: %s with error: %s", s.Driver, s.Host, err)
		}
	}
	return s.Conn, nil
}

func (s *DbServerConnection) Close() error {
	if s.Conn != nil {
		err := s.Conn.Close()
		s.Conn = nil
		return err
	}
	return nil
}
