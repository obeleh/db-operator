package postgres

import (
	"fmt"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/query_utils"
	"github.com/obeleh/db-operator/shared"
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
	// sslmode=disable
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		p.Host, p.Port, p.UserName, p.Password, p.Database)
}

func (p *PostgresConnection) CreateUser(userName string, password string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`CREATE USER %q LOGIN PASSWORD '%s';`, userName, password))
	return err
}

func (p *PostgresConnection) DropUser(userName string) error {
	conn, err := p.GetDbConnection()
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("DROP USER IF EXISTS %q;", userName))
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
	_, err = conn.Exec(fmt.Sprintf("DROP DATABASE %q;", dbName))
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

func (p *PostgresConnection) GetBackupJobs() ([]map[string]interface{}, error) {
	conn, err := p.GetDbConnection()
	if err != nil {
		return nil, err
	}

	return shared.SelectToArrayMap(conn, "WITH x as (SHOW JOBS) SELECT * FROM x WHERE job_type = 'BACKUP' ORDER BY created DESC LIMIT 100;")
}

func (p *PostgresConnection) GetBackupJobById(jobId string) (map[string]interface{}, bool, error) {
	conn, err := p.GetDbConnection()
	if err != nil {
		return nil, false, err
	}

	maps, err := shared.SelectToArrayMap(conn, "WITH x as (SHOW JOBS) SELECT * FROM x WHERE job_type = 'BACKUP' AND job_id=$1;", jobId)
	if err != nil {
		return nil, false, err
	}

	if len(maps) == 0 {
		return nil, false, nil
	}

	return maps[0], true, nil
}

func (p *PostgresConnection) CreateBackupJob(dbName string, bucketSecret string, bucketStorageInfo shared.BucketStorageInfo) (string, error) {
	conn, err := p.GetDbConnection()
	if err != nil {
		return "", err
	}

	/*
		BACKUP INTO {'subdirectory'} IN 's3://{BUCKET NAME}?AWS_ACCESS_KEY_ID={KEY ID}&AWS_SECRET_ACCESS_KEY={SECRET ACCESS KEY}' \
		AS OF SYSTEM TIME '-10s';
	*/

	qry := "BACKUP INTO"
	if len(bucketStorageInfo.Prefix) > 0 {
		qry += fmt.Sprintf(" {'%s'}", bucketStorageInfo.Prefix)
	}

	qry += " IN '%s://{BUCKET NAME}?AWS_ACCESS_KEY_ID={KEY ID}&AWS_SECRET_ACCESS_KEY={SECRET ACCESS KEY}'"
	qry += " WITH DETACHED;"

	return query_utils.SelectFirstValueString(conn, qry)
}
