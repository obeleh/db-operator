package mysql

import (
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
)

type MySqlConnection struct {
	shared.DbServerConnection
}

func (m *MySqlConnection) GetConnectionString() string {
	// "username:password@tcp(127.0.0.1:3306)/test"
	if m.Password == nil {
		panic("Not implemented nil password for mysql")
		//TODO: https://stackoverflow.com/questions/67109556/connect-to-mysql-mariadb-with-ssl-and-certs-in-go
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", m.UserName, *m.Password, m.Host, m.Port, m.Database)
}

func (m *MySqlConnection) CreateUser(userName string, password string) error {
	conn, err := m.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`CREATE USER '%s'@'%%' IDENTIFIED BY '%s';`, userName, password))
	return err
}

func (m *MySqlConnection) SelectToArrayMap(query string) ([]map[string]interface{}, error) {
	conn, err := m.GetDbConnection()
	if err != nil {
		return nil, err
	}
	return shared.SelectToArrayMap(conn, query)
}

func (m *MySqlConnection) DropUser(userName string) error {
	conn, err := m.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`DROP USER '%s'@'%%';`, userName))
	return err
}

func (m *MySqlConnection) GetUsers() (map[string]shared.DbSideUser, error) {
	conn, err := m.GetDbConnection()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query("select user, host from mysql.user;")
	if err != nil {
		return nil, fmt.Errorf("unable to read users from server")
	}

	users := make(map[string]shared.DbSideUser)

	for rows.Next() {
		var dbUser shared.DbSideUser
		err := rows.Scan(&dbUser.UserName, &dbUser.Attributes)
		if err != nil {
			return nil, fmt.Errorf("unable to load DbUser")
		}
		if !strings.HasPrefix(dbUser.UserName, "mysql.") {
			users[dbUser.UserName] = dbUser
		}
	}
	return users, nil
}

func (m *MySqlConnection) Execute(qry string) error {
	conn, err := m.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(qry)
	return err
}

func (m *MySqlConnection) CreateDb(dbName string) error {
	return m.Execute(fmt.Sprintf("CREATE DATABASE `%s`;", dbName))
}

func (m *MySqlConnection) DropDb(dbName string) error {
	conn, err := m.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("DROP DATABASE `%s`;", dbName))
	return err
}

func ToString(val interface{}) string {
	return string(val.([]uint8))
}

func (m *MySqlConnection) GetDbs() (map[string]shared.DbSideDb, error) {
	conn, err := m.GetDbConnection()
	if err != nil {
		return nil, err
	}

	databases := map[string]shared.DbSideDb{}

	rows, err := conn.Query("SELECT DISTINCT SCHEMA_NAME AS `database` FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys') ORDER BY SCHEMA_NAME;")
	if err != nil {
		return nil, fmt.Errorf("unable to read databases from server %s", err)
	}
	for rows.Next() {
		var db string
		err := rows.Scan(&db)
		if err != nil {
			return nil, fmt.Errorf("unable to load Db: %s", err)
		}

		databases[db] = shared.DbSideDb{DatbaseName: db}
	}

	return databases, nil
}

func (m *MySqlConnection) Close() error {
	if m.Conn != nil {
		err := m.Conn.Close()
		m.Conn = nil
		return err
	}
	return nil
}

func (p *MySqlConnection) UpdateUserPrivs(userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv) (bool, error) {
	conn, err := p.GetDbConnection()
	if err != nil {
		return false, err
	}

	return UpdateUserPrivs(conn, userName, serverPrivs, dbPrivs)
}

func (p *MySqlConnection) ScopeToDbName(scope string) (string, error) {
	parts := strings.Split(scope, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("Unexpected scope found expected form db.something, db.* or *.*, got %s", scope)
	}
	return parts[0], nil
}
