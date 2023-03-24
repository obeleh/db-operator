package shared

import (
	"database/sql"
	"fmt"
)

type Connector interface {
	Connect(connectInfo *DbServerConnectInfo) (*sql.DB, error)
}

type ConnectionsStore struct {
	connectInfos map[string]*DbServerConnectInfo
	connections  map[string]*sql.DB
	Connector
}

func (c *ConnectionsStore) GetDbConnection(connectionName string) (*sql.DB, error) {
	conn, found := c.connections[connectionName]
	if !found {
		connectInfo, found := c.connectInfos[connectionName]
		if !found {
			return nil, fmt.Errorf("Connection with name '%s' not found", connectionName)
		}
		conn, err := c.Connect(connectInfo)
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
