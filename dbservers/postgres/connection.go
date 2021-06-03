package postgres

import (
	"fmt"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	"github.com/kabisa/db-operator/shared"
)

const PG_GRANT_SCRIPT string = `
GRANT USAGE ON SCHEMA public TO "%s";
GRANT ALL ON ALL TABLES IN SCHEMA public TO "%s";
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO "%s";
ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public
GRANT ALL ON TABLES TO "%s";
ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public
GRANT ALL ON SEQUENCES TO "%s";
ALTER DATABASE "%s" OWNER TO "%s";
`

type PostgresConnection struct {
	shared.DbServerConnection
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
	_, err = conn.Exec(fmt.Sprintf("DROP ROLE IF EXISTS %q;", userName))
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

func (p *PostgresConnection) UpdateUserPrivs(userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv) (bool, error) {
	conn, err := p.GetDbConnection()
	if err != nil {
		return false, err
	}
	return UpdateUserPrivs(conn, userName, serverPrivs, dbPrivs)
}

func (p *PostgresConnection) GetUsers() (map[string]shared.DbSideUser, error) {
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
		ORDER BY usename ASC;`,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to read users from server %s", err)
	}

	users := make(map[string]shared.DbSideUser)

	for rows.Next() {
		var dbUser shared.DbSideUser
		err := rows.Scan(&dbUser.UserName, &dbUser.Attributes)
		if err != nil {
			return nil, fmt.Errorf("unable to load DbUser")
		}
		users[dbUser.UserName] = dbUser
	}
	return users, nil
}

func (p *PostgresConnection) CreateDb(dbName string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("CREATE DATABASE %q;", dbName))
	return err
}

func (p *PostgresConnection) DropDb(dbName string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec("DROP DATABASE %q;", dbName)
	return err
}

func (p *PostgresConnection) GetDbs() (map[string]shared.DbSideDb, error) {
	conn, err := p.GetDbConnection()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query("SELECT d.datname FROM pg_catalog.pg_database d WHERE d.datistemplate = false ORDER BY d.datname;")
	if err != nil {
		return nil, err
	}

	databases := make(map[string]shared.DbSideDb)

	for rows.Next() {
		var database shared.DbSideDb
		err := rows.Scan(&database.DatbaseName)
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
