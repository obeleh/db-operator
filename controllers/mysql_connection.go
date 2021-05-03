package controllers

// https://github.com/ansible-collections/community.mysql/blob/main/plugins/modules/mysql_user.py

import (
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type MySqlConnection struct {
	DbServerConnection
}

func (m *MySqlConnection) GetConnectionString() string {
	// "username:password@tcp(127.0.0.1:3306)/test"
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", m.UserName, m.Password, m.Host, m.Port, m.Database)
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

	rows, _ := conn.Query(query)
	cols, _ := rows.Columns()

	rowMaps := make([]map[string]interface{}, 0)

	for rows.Next() {
		// Create a slice of interface{}'s to represent each column,
		// and a second slice to contain pointers to each item in the columns slice.
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		// Scan the result into the column pointers...
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		// Create our map, and retrieve the value for each column from the pointers slice,
		// storing it in the map with the name of the column as the key.
		m := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			m[colName] = *val
		}
		rowMaps = append(rowMaps, m)
	}

	return rowMaps, nil
}

func (m *MySqlConnection) DropUser(userName string) error {
	conn, err := m.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`DROP USER '%s'@'%%';`, userName))
	return err
}

func (m *MySqlConnection) MakeUserDbOwner(userName string, dbName string) error {
	conn, err := m.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%%';`, dbName, userName))
	return err
}

func (m *MySqlConnection) GetUsers() (map[string]DbSideUser, error) {
	conn, err := m.GetDbConnection()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query("select user, host from mysql.user;")
	if err != nil {
		return nil, fmt.Errorf("Unable to read users from server")
	}

	users := make(map[string]DbSideUser)

	for rows.Next() {
		var dbUser DbSideUser
		err := rows.Scan(&dbUser.UserName, &dbUser.Attributes)
		if err != nil {
			return nil, fmt.Errorf("unable to load DbUser")
		}
		users[dbUser.UserName] = dbUser
	}
	return users, nil
}

func (m *MySqlConnection) CreateDb(dbName string, dbOwner string) error {
	conn, err := m.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("CREATE DATABASE %s; ", dbName))
	if err != nil {
		return err
	}
	return m.MakeUserDbOwner(dbOwner, dbName)
}

func (m *MySqlConnection) DropDb(dbName string) error {
	conn, err := m.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("DROP DATABASE %s;", dbName))
	return err
}

func (m *MySqlConnection) GetDbs() (map[string]DbSideDb, error) {
	conn, err := m.GetDbConnection()
	if err != nil {
		return nil, err
	}

	dbArrMap, err := m.SelectToArrayMap("select * from mysql.db where  db  NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys') and Host = '%';")
	if err != nil {
		return nil, err
	}

	databases := make(map[string]DbSideDb)

ROWS:
	for _, row := range dbArrMap {

		for ky, vl := range row {
			if ky == "Db" || ky == "Host" || ky == "User" || strings.HasPrefix(ky, "Grant_") {
				continue
			}
			if vl.(string) != "Y" {
				continue ROWS
			}
		}

		db := row["Db"].(string)
		user := row["User"].(string)

		databases[db] = DbSideDb{DatbaseName: db, Owner: user}
	}

	rows, err := conn.Query("SELECT DISTINCT SCHEMA_NAME AS `database` FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys') ORDER BY SCHEMA_NAME;")
	if err != nil {
		return nil, fmt.Errorf("Unable to read databases from server")
	}
	for rows.Next() {
		var db string
		err := rows.Scan(&db)
		if err != nil {
			return nil, fmt.Errorf("unable to load Db: %s", err)
		}

		if _, exists := databases[db]; !exists {
			databases[db] = DbSideDb{DatbaseName: db, Owner: ""}
		}
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
