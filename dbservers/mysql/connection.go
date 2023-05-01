package mysql

import (
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/shared"
)

type MySqlConnection struct {
	shared.ConnectionsStore
}

func NewMySqlConnection(connectionInfo *shared.DbServerConnectInfo, userCredentials map[string]*shared.Credentials) *MySqlConnection {
	return &MySqlConnection{
		ConnectionsStore: shared.ConnectionsStore{
			ServerConnInfo:  connectionInfo,
			UserCredentials: userCredentials,
			Connector:       &MySqlConnector{},
		},
	}
}

func (m *MySqlConnection) CreateUser(userName string, password string) error {
	conn, err := m.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`CREATE USER '%s'@'%%' IDENTIFIED BY '%s';`, userName, password))
	return err
}

func (m *MySqlConnection) SelectToArrayMap(query string) ([]map[string]interface{}, error) {
	conn, err := m.GetDbConnection(nil, nil)
	if err != nil {
		return nil, err
	}
	return shared.SelectToArrayMap(conn, query)
}

func (m *MySqlConnection) DropUser(userSpec dboperatorv1alpha1.UserSpec) error {
	if userSpec.DropUserOptions != nil {
		return fmt.Errorf("DROP USER options not supported (yet?) for MySQL")
	}
	conn, err := m.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`DROP USER '%s'@'%%';`, userSpec.UserName))
	return err
}

func (m *MySqlConnection) GetUsers() (map[string]shared.DbSideUser, error) {
	conn, err := m.GetDbConnection(nil, nil)
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query("select user, host from mysql.user;")
	if err != nil {
		return nil, fmt.Errorf("unable to read users from server")
	}
	defer rows.Close()

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

func (m *MySqlConnection) Execute(query string, userName *string) error {
	conn, err := m.GetDbConnection(userName, nil)
	if err != nil {
		return err
	}
	_, err = conn.Exec(query)
	return err
}

func (m *MySqlConnection) CreateDb(dbName string) error {
	return m.Execute(fmt.Sprintf("CREATE DATABASE `%s`;", dbName), nil)
}

func (m *MySqlConnection) DropDb(dbName string, cascade bool) error {
	conn, err := m.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}
	if cascade {
		return fmt.Errorf("CASCADE option not posible for MySQL")
	}
	_, err = conn.Exec(fmt.Sprintf("DROP DATABASE `%s`;", dbName))
	return err
}

func ToString(val interface{}) string {
	return string(val.([]uint8))
}

func (m *MySqlConnection) GetDbs() (map[string]shared.DbSideDb, error) {
	conn, err := m.GetDbConnection(nil, nil)
	if err != nil {
		return nil, err
	}

	databases := map[string]shared.DbSideDb{}

	rows, err := conn.Query("SELECT DISTINCT SCHEMA_NAME AS `database` FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys') ORDER BY SCHEMA_NAME;")
	if err != nil {
		return nil, fmt.Errorf("unable to read databases from server %s", err)
	}
	defer rows.Close()

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

func (m *MySqlConnection) GetSchemas(userName *string) (map[string]shared.DbSideSchema, error) {
	return nil, fmt.Errorf("TODO check if there is a difference between schemas and dbs in MySQL")
}

func (m *MySqlConnection) CreateSchema(schemaName string, creator *string) error {
	return fmt.Errorf("TODO check if there is a difference between schemas and dbs in MySQL")
}

func (m *MySqlConnection) DropSchema(schemaName string, userName *string, cascade bool) error {
	return fmt.Errorf("TODO check if there is a difference between schemas and dbs in MySQL")
}

func (p *MySqlConnection) UpdateUserPrivs(userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv) (bool, error) {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return false, err
	}

	return UpdateUserPrivs(conn, userName, serverPrivs, dbPrivs)
}

func quoteMySQLIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}
