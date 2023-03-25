package shared

import (
	"database/sql"
	"fmt"
)

type Connector interface {
	Connect(connectInfo *DbServerConnectInfo, credentials *Credentials) (*sql.DB, error)
}

type ConnectionsStore struct {
	serverConnInfo  *DbServerConnectInfo
	userCredentials map[string]*Credentials
	connections     map[string]*sql.DB
	Connector
}

func (c *ConnectionsStore) GetDbConnection(connectionName string) (*sql.DB, error) {
	conn, found := c.connections[connectionName]
	if !found {
		var creds *Credentials
		if connectionName != "" {
			creds, found = c.userCredentials[connectionName]
			if !found {
				return nil, fmt.Errorf("Connection with name '%s' not found", connectionName)
			}
		}
		conn, err := c.Connect(c.serverConnInfo, creds)
		if err != nil {
			return nil, err
		}
		c.connections[connectionName] = conn
	}
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
