package shared

import (
	"database/sql"
	"fmt"
)

type Connector interface {
	Connect(connectInfo *DbServerConnectInfo, credentials *Credentials, databaseName *string) (*sql.DB, error)
}

type ConnectionKey struct {
	DbName, UserName string
}

type ConnectionsStore struct {
	ServerConnInfo  *DbServerConnectInfo
	UserCredentials map[string]*Credentials
	connections     map[ConnectionKey]*sql.DB
	Connector
}

func NewConnectionKey(userName, databaseName *string) ConnectionKey {
	userNameForKey := ""
	if userName != nil {
		userNameForKey = *userName
	}

	databaseNameForKey := ""
	if databaseName != nil {
		databaseNameForKey = *databaseName
	}

	return ConnectionKey{
		UserName: userNameForKey,
		DbName:   databaseNameForKey,
	}
}

func (c *ConnectionsStore) GetDbConnection(userName, databaseName *string) (*sql.DB, error) {
	var creds *Credentials
	connectionKey := NewConnectionKey(userName, databaseName)

	conn, found := c.connections[connectionKey]
	if found {
		return conn, nil
	}

	if userName == nil {
		userName = &c.ServerConnInfo.UserName
		creds = &c.ServerConnInfo.Credentials
	} else {
		creds, found = c.UserCredentials[*userName]
		if !found {
			return nil, fmt.Errorf("Credentials for '%s' not found while constructing database connection", userName)
		}
	}
	conn, err := c.Connect(c.ServerConnInfo, creds, databaseName)
	if err != nil {
		return nil, err
	}

	if c.connections == nil {
		c.connections = make(map[ConnectionKey]*sql.DB)
	}
	c.connections[connectionKey] = conn
	return conn, nil
}

func (c *ConnectionsStore) Close() error {
	for _, conn := range c.connections {
		err := conn.Close()
		if err != nil {
			return err
		}
	}
	c.connections = nil
	return nil
}
