package controllers

// https://github.com/ansible-collections/community.postgresql/blob/main/plugins/modules/postgresql_user.py

import (
	"fmt"
)

type PostgresConnection struct {
	DbServerConnection
}

func (p *PostgresConnection) GetConnectionString() string {
	if len(p.Database) == 0 {
		panic("No database configured")
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		p.Host, p.Port, p.UserName, p.Password, p.Database)
}

func (p *PostgresConnection) CreateUser(userName string, password string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s';`, userName, password))
	return err
}

func (p *PostgresConnection) DropUser(userName string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`DROP ROLE IF EXISTS %s;`, userName))
	return err
}

func (p *PostgresConnection) MakeUserDbOwner(userName string, dbName string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(PG_GRANT_SCRIPT, userName, userName, userName, userName, userName, userName, userName, dbName, userName))
	return err
}

func (p *PostgresConnection) GetUsers() (map[string]DbSideUser, error) {
	conn, err := p.GetDbConnection()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query(
		`SELECT usename AS role_name,
			CASE 
			WHEN usesuper AND usecreatedb THEN 
				CAST('superuser, create database' AS pg_catalog.text)
			WHEN usesuper THEN 
				CAST('superuser' AS pg_catalog.text)
			WHEN usecreatedb THEN 
				CAST('create database' AS pg_catalog.text)
			ELSE 
				CAST('' AS pg_catalog.text)
			END role_attributes
		FROM pg_catalog.pg_user
		ORDER BY role_name desc;`,
	)
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

func (p *PostgresConnection) CreateDb(dbName string, dbOwner string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("CREATE DATABASE %q WITH OWNER = '%s';", dbName, dbOwner))
	return err
}

func (p *PostgresConnection) DropDb(dbName string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("DROP DATABASE %q;", dbName))
	return err
}

func (p *PostgresConnection) GetDbs() (map[string]DbSideDb, error) {
	conn, err := p.GetDbConnection()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query("SELECT d.datname, pg_catalog.pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datistemplate = false;")
	if err != nil {
		return nil, err
	}

	databases := make(map[string]DbSideDb)

	for rows.Next() {
		var database DbSideDb
		err := rows.Scan(&database.DatbaseName, &database.Owner)
		if err != nil {
			return nil, fmt.Errorf("unable to load PostgresDb")
		}
		databases[database.DatbaseName] = database
	}
	return databases, nil
}

func (p *PostgresConnection) Close() error {
	if p.Conn != nil {
		err := p.Conn.Close()
		p.Conn = nil
		return err
	}
	return nil
}
