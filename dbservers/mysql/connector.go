package mysql

import (
	"database/sql"
	"fmt"

	"github.com/obeleh/db-operator/shared"
)

type MySqlConnector struct {
}

func (m *MySqlConnector) Connect(connectInfo *shared.DbServerConnectInfo, credentials *shared.Credentials) (*sql.DB, error) {
	// "username:password@tcp(127.0.0.1:3306)/test"
	var connStr string
	if credentials == nil {
		if connectInfo.Password == nil {
			return nil, fmt.Errorf("Passwordless connections not yet implemented for MySQL connections")
			//TODO: https://stackoverflow.com/questions/67109556/connect-to-mysql-mariadb-with-ssl-and-certs-in-go
		}
		connStr = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			connectInfo.UserName,
			*connectInfo.Password,
			connectInfo.Host,
			connectInfo.Port,
			connectInfo.Database,
		)
	} else {
		if credentials.Password == nil {
			return nil, fmt.Errorf("Passwordless connections not yet implemented for MySQL connections")
			//TODO: https://stackoverflow.com/questions/67109556/connect-to-mysql-mariadb-with-ssl-and-certs-in-go
		}
		connStr = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			credentials.UserName,
			*credentials.Password,
			connectInfo.Host,
			connectInfo.Port,
			connectInfo.Database,
		)
	}

	return sql.Open("mysql", connStr)
}
