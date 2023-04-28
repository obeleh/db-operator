package postgres

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/lib/pq"

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
	shared.ConnectionsStore
}

func NewPostgresConnection(connectionInfo *shared.DbServerConnectInfo, userCredentials map[string]*shared.Credentials) *PostgresConnection {
	return &PostgresConnection{
		ConnectionsStore: shared.ConnectionsStore{
			ServerConnInfo:  connectionInfo,
			UserCredentials: userCredentials,
			Connector:       &PostgresConnector{},
		},
	}
}

func (p *PostgresConnection) CreateUser(userName string, password string) error {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}

	quotedUserName := pq.QuoteIdentifier(userName)
	quotedPassword := pq.QuoteLiteral(password)
	_, err = conn.Exec(fmt.Sprintf(`CREATE USER %s LOGIN PASSWORD %s;`, quotedUserName, quotedPassword))
	return err
}

func (p *PostgresConnection) DropUser(userName string) error {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}

	quotedUserName := pq.QuoteIdentifier(userName)
	_, err = conn.Exec(fmt.Sprintf("DROP USER IF EXISTS %s;", quotedUserName))
	return err
}

func (p *PostgresConnection) MakeUserDbOwner(userName string, dbName string) error {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(PG_GRANT_SCRIPT, userName, userName, userName, userName, userName, userName, userName, dbName, userName))
	return err
}

func (p *PostgresConnection) UpdateUserPrivs(userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv) (bool, error) {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return false, err
	}
	return UpdateUserPrivs(conn, userName, serverPrivs, dbPrivs, p.GetDbConnection)
}

func (p *PostgresConnection) GetUsers() (map[string]shared.DbSideUser, error) {
	conn, err := p.GetDbConnection(nil, nil)
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
	defer rows.Close()

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

func (p *PostgresConnection) Execute(qry string, user *string) error {
	conn, err := p.GetDbConnection(user, nil)
	if err != nil {
		return err
	}
	_, err = conn.Exec(qry)
	return err
}

func (p *PostgresConnection) CreateDb(dbName string) error {
	quotedDbName := pq.QuoteIdentifier(dbName)
	return p.Execute(fmt.Sprintf("CREATE DATABASE %s;", quotedDbName), nil)
}

func (p *PostgresConnection) CreateSchema(schemaName string, creator *string) error {
	quotedSchemaName := pq.QuoteIdentifier(schemaName)
	return p.Execute(fmt.Sprintf("CREATE SCHEMA %s;", quotedSchemaName), creator)
}

func (p *PostgresConnection) DropDb(dbName string, cascade bool) error {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}
	cascadeStr := ""
	if cascade {
		cascadeStr = "CASCADE"
	}
	quotedDbName := pq.QuoteIdentifier(dbName)
	_, err = conn.Exec(fmt.Sprintf("DROP DATABASE %s %s;", quotedDbName, cascadeStr))
	return err
}

func (p *PostgresConnection) DropSchema(schemaName string, userName *string, cascade bool) error {
	var dbName *string
	if strings.Contains(schemaName, ".") {
		dbNameStr := GetDbNameFromScopeName(schemaName)
		dbName = &dbNameStr
	}
	conn, err := p.GetDbConnection(userName, dbName)
	if err != nil {
		return err
	}
	cascadeStr := ""
	if cascade {
		cascadeStr = "CASCADE"
	}
	quotedSchemaName := pq.QuoteIdentifier(schemaName)
	_, err = conn.Exec(fmt.Sprintf("DROP SCHEMA %s %s;", quotedSchemaName, cascadeStr))
	return err
}

func (p *PostgresConnection) GetDbs() (map[string]shared.DbSideDb, error) {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query("SELECT d.datname FROM pg_catalog.pg_database d WHERE d.datistemplate = false ORDER BY d.datname;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func (p *PostgresConnection) GetSchemas(userName *string) (map[string]shared.DbSideSchema, error) {
	conn, err := p.GetDbConnection(userName, nil)
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query("SELECT nspname FROM pg_catalog.pg_namespace WHERE nspname NOT IN ('crdb_internal', 'information_schema', 'pg_catalog', 'pg_extension');")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schemas := make(map[string]shared.DbSideSchema)

	for rows.Next() {
		var schema shared.DbSideSchema
		err := rows.Scan(&schema.SchemaName)
		if err != nil {
			return nil, fmt.Errorf("unable to load PostgresDb")
		}
		schemas[schema.SchemaName] = schema
	}
	return schemas, nil
}

func (p *PostgresConnection) GetBackupJobs() ([]map[string]interface{}, error) {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return nil, err
	}

	return shared.SelectToArrayMap(conn, "WITH x as (SHOW JOBS) SELECT * FROM x WHERE job_type = 'BACKUP' ORDER BY created DESC LIMIT 100;")
}

func (p *PostgresConnection) GetBackupJobById(jobId int64) (map[string]interface{}, bool, error) {
	conn, err := p.GetDbConnection(nil, nil)
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

func (p *PostgresConnection) GetBackupScheduleById(scheduleId int64) (map[string]interface{}, error) {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return nil, err
	}

	maps, err := shared.SelectToArrayMap(conn, "WITH x as (SHOW SCHEDULES) SELECT * FROM x WHERE id=$1;", scheduleId)
	if err != nil {
		return nil, err
	}

	if len(maps) == 0 {
		return nil, fmt.Errorf(fmt.Sprintf("BackupSchedule with id %d not found", scheduleId))
	}

	return maps[0], nil
}

func (p *PostgresConnection) GetBackupScheduleByLabel(scheduleLabel string) (map[string]interface{}, error) {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return nil, err
	}

	maps, err := shared.SelectToArrayMap(conn, "WITH x as (SHOW SCHEDULES) SELECT * FROM x WHERE label=$1;", scheduleLabel)
	if err != nil {
		return nil, err
	}

	if len(maps) == 0 {
		return nil, fmt.Errorf(fmt.Sprintf("BackupSchedule with label %s not found", scheduleLabel))
	}

	return maps[0], nil
}

func (p *PostgresConnection) ResumeSchedule(scheduleId int64) error {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("RESUME SCHEDULE %d", scheduleId))
	return err
}

func (p *PostgresConnection) PauseSchedule(scheduleId int64) error {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf("PAUSE SCHEDULE %d", scheduleId))
	return err
}

func (p *PostgresConnection) CreateBackupJob(dbName string, bucketStorageInfo shared.BucketStorageInfo) (int64, error) {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return 0, err
	}

	/*
		BACKUP DATABASE "database" \
		INTO '{bucketstring}' \
		AS OF SYSTEM TIME '-10s';
	*/

	// BACKUP DATABASE "example-db" INTO 's3://testbucket?AWS_ACCESS_KEY_ID=MYKEY&AWS_ENDPOINT=http%3A%2F%2Fminio.default.svc.cluster.local%3A9000&AWS_SECRET_ACCESS_KEY=MYSECRET' AS OF SYSTEM TIME '-10s' WITH detached;
	qry, err := p.ConstructBackupJobStatement(bucketStorageInfo, dbName, "AS OF SYSTEM TIME '-10s' ", false)
	if err != nil {
		return 0, err
	}
	return query_utils.SelectFirstValueInt64(conn, qry)
}

func (*PostgresConnection) ConstructBackupJobStatement(bucketStorageInfo shared.BucketStorageInfo, dbName string, systemTimeOffsetStr string, redact bool) (string, error) {
	bucketString, err := getBucketString(bucketStorageInfo, redact)
	if err != nil {
		return "", err
	}

	quotedDbName := pq.QuoteIdentifier(dbName)
	qry := fmt.Sprintf(
		"BACKUP DATABASE %s INTO '%s' %sWITH detached;",
		quotedDbName,
		bucketString,
		systemTimeOffsetStr,
	)
	return qry, nil
}

func (p *PostgresConnection) CreateBackupSchedule(dbName string, bucketStorageInfo shared.BucketStorageInfo, scheduleName, schedule string, runNow bool, ignoreExistingBackups bool) ([]map[string]interface{}, error) {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return nil, err
	}

	bucketString, err := getBucketString(bucketStorageInfo, false)
	if err != nil {
		return nil, err
	}
	/*
		CREATE SCHEDULE IF NOT EXISTS "scheduleName" FOR BACKUP DATABASE "database"
		INTO '{bucketstring}'
		RECURRING '{schedule}' FULL BACKUP
		ALWAYS WITH SCHEDULE OPTIONS first_run=now;
	*/

	escapedScheduleName := pq.QuoteIdentifier(scheduleName)
	escapedDbName := pq.QuoteIdentifier(dbName)
	escapedBucketString := pq.QuoteLiteral(bucketString)
	escapedSchedule := pq.QuoteLiteral(schedule)

	qry := fmt.Sprintf(
		"CREATE SCHEDULE IF NOT EXISTS %s FOR BACKUP DATABASE %s INTO %s RECURRING %s FULL BACKUP ALWAYS",
		escapedScheduleName,
		escapedDbName,
		escapedBucketString,
		escapedSchedule,
	)
	if runNow || ignoreExistingBackups {
		qry += " WITH SCHEDULE OPTIONS"

		if runNow {
			qry += " first_run=now"
		}
		if ignoreExistingBackups {
			qry += ",ignore_existing_backups"
		}
	}
	qry += ";"
	return shared.SelectToArrayMap(conn, qry)
}

func (p *PostgresConnection) DropBackupSchedule(scheduleId int64) error {
	conn, err := p.GetDbConnection(nil, nil)
	if err != nil {
		return err
	}

	_, err = conn.Exec(fmt.Sprintf("DROP SCHEDULE %d", scheduleId))
	return err
}

func getBucketString(bucketStorageInfo shared.BucketStorageInfo, redact bool) (string, error) {
	u := &url.URL{
		Scheme: bucketStorageInfo.StorageTypeName,
		Host:   bucketStorageInfo.BucketName,
	}

	if bucketStorageInfo.Prefix != "" {
		u.Path = bucketStorageInfo.Prefix
	}

	query := url.Values{}

	if bucketStorageInfo.KeyName != "" {
		bucketSecret, err := bucketStorageInfo.GetBucketSecret()
		if err != nil {
			return "", err
		}
		query.Set("AWS_ACCESS_KEY_ID", bucketStorageInfo.KeyName)
		if redact {
			bucketSecret = "redacted"
		}
		query.Set("AWS_SECRET_ACCESS_KEY", bucketSecret)
	} else {
		query.Set("AUTH", "implicit")
	}

	if bucketStorageInfo.Endpoint != "" {
		query.Set("AWS_ENDPOINT", bucketStorageInfo.Endpoint)
	}

	u.RawQuery = query.Encode()

	return u.String(), nil
}
