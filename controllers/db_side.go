package controllers

import (
	"database/sql"
	"fmt"
)

type DbSideUser struct {
	UserName   string
	Attributes string
}

type DbSideDb struct {
	DatbaseName string
	Owner       string
}

type DbServerConnectInfo struct {
	Host     string
	Port     int
	UserName string
	Password string
	Database string
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
	MakeUserDbOwner(userName string, dbName string) error
	GetUsers() (map[string]DbSideUser, error)
	CreateDb(dbName string, dbOwner string) error
	DropDb(dbName string) error
	GetDbs() (map[string]DbSideDb, error)
	Close() error
}

func (s *DbServerConnection) GetDbConnection() (*sql.DB, error) {
	if s.Conn == nil {
		var err error
		connStr := s.GetConnectionString()
		s.Conn, err = sql.Open(s.Driver, connStr)
		if err != nil {
			return nil, fmt.Errorf("Failed to open a %s DB connection to: %s", s.Driver, s.Host)
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
