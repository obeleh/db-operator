package shared

import (
	"database/sql"
	"fmt"
)

type DbServerConnection struct {
	DbServerConnectInfo
	Conn   *sql.DB
	Driver string
	DbServerConnectionInterface
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
