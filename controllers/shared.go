package controllers

import (
	"context"
	"database/sql"
	"fmt"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	_ "github.com/lib/pq"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PostgresUser struct {
	UserName   string
	Attributes string
}

type PostgresDb struct {
	DatbaseName string
	Owner       string
}

type PostgresConnectInfo struct {
	Host     string
	Port     int
	UserName string
	Password string
	Database string
}

func NewPostgresConnectInfo(host string, port int, userName string, password string, database *string) PostgresConnectInfo {
	var dbName string
	if database == nil {
		dbName = "postgres"
	} else {
		dbName = *database
	}
	return PostgresConnectInfo{
		Host:     host,
		Port:     port,
		UserName: userName,
		Password: password,
		Database: dbName,
	}
}

func (p *PostgresConnectInfo) GetConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		p.Host, p.Port, p.UserName, p.Password, p.Database)
}

func (p *PostgresConnectInfo) GetDbConnection() (*sql.DB, error) {
	pgDbServer, err := sql.Open("postgres", p.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("Failed to open a DB connection to: %s", p.Host)
	}
	return pgDbServer, nil
}

func GetPgConnectInfo(k8sClient client.Client, ctx context.Context, dbServer *dboperatorv1alpha1.DbServer, dbName *string) (*PostgresConnectInfo, error) {
	secretName := types.NamespacedName{
		Name:      dbServer.Spec.SecretName,
		Namespace: dbServer.Namespace,
	}
	secret := &v1.Secret{}

	err := k8sClient.Get(ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("Failed to get secret: %s", dbServer.Spec.SecretName)
	}

	connectInfo := NewPostgresConnectInfo(dbServer.Spec.Address, dbServer.Spec.Port, dbServer.Spec.UserName, string(secret.Data[dbServer.Spec.SecretKey]), dbName)
	return &connectInfo, nil
}

func GetDbConnectionInfoFromServerName(k8sClient client.Client, ctx context.Context, serverName string, namespace string, dbName *string) (*PostgresConnectInfo, error) {
	serverNsName := types.NamespacedName{
		Name:      serverName,
		Namespace: namespace,
	}
	dbServer := &dboperatorv1alpha1.DbServer{}

	err := k8sClient.Get(ctx, serverNsName, dbServer)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Server: %s", serverName)
	}

	info, err := GetPgConnectInfo(k8sClient, ctx, dbServer, dbName)
	return info, err
}

func CreateDb(dbName string, dbOwner string, dbServerConn *sql.DB) error {
	_, err := dbServerConn.Exec(fmt.Sprintf("CREATE DATABASE %q WITH OWNER = '%s';", dbName, dbOwner))
	return err
}

func DropPgDb(dbName string, dbServerConn *sql.DB) error {
	_, err := dbServerConn.Exec(fmt.Sprintf("DROP DATABASE %q;", dbName))
	return err
}

func GetDbs(dbServerConn *sql.DB) (map[string]PostgresDb, error) {

	rows, err := dbServerConn.Query("SELECT d.datname, pg_catalog.pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datistemplate = false;")
	if err != nil {
		return nil, fmt.Errorf("Unable to read databases from server")
	}

	databases := make(map[string]PostgresDb)

	for rows.Next() {
		var database PostgresDb
		err := rows.Scan(&database.DatbaseName, &database.Owner)
		if err != nil {
			return nil, fmt.Errorf("unable to load PostgresDb")
		}
		databases[database.DatbaseName] = database
	}
	return databases, nil
}

func GetUsers(dbServerConn *sql.DB) (map[string]PostgresUser, error) {
	rows, err := dbServerConn.Query(
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

	postgresUsers := make(map[string]PostgresUser)

	for rows.Next() {
		var postgresUser PostgresUser
		err := rows.Scan(&postgresUser.UserName, &postgresUser.Attributes)
		if err != nil {
			return nil, fmt.Errorf("unable to load PostgresUser")
		}
		postgresUsers[postgresUser.UserName] = postgresUser
	}
	return postgresUsers, nil
}

func GetUserPassword(dbUser *dboperatorv1alpha1.User, k8sClient client.Client, ctx context.Context) (*string, error) {
	secretName := types.NamespacedName{
		Name:      dbUser.Spec.SecretName,
		Namespace: dbUser.Namespace,
	}
	secret := &v1.Secret{}
	err := k8sClient.Get(ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("Failed to get secret: %s", dbUser.Spec.SecretName)
	}

	var passwordKey string
	if len(dbUser.Spec.SecretKey) == 0 {
		passwordKey = "password"
	} else {
		passwordKey = dbUser.Spec.SecretKey
	}

	passBytes, ok := secret.Data[passwordKey]
	if !ok {
		return nil, fmt.Errorf("Password key (%s) not found in secret", passwordKey)
	}

	password := string(passBytes)

	return &password, nil
}

func CreatePgUser(userName string, password string, dbServerConn *sql.DB) error {
	_, err := dbServerConn.Exec(fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s';`, userName, password))
	return err
}

func DropPgUser(userName string, dbServerConn *sql.DB) error {
	_, err := dbServerConn.Exec(fmt.Sprintf(`DROP ROLE IF EXISTS %s;`, userName))
	return err
}

func MakeUserDbOwner(userName string, dbName string, dbServerConn *sql.DB) error {
	_, err := dbServerConn.Exec(fmt.Sprintf(`
	GRANT USAGE ON SCHEMA public TO "%s";
	GRANT ALL ON ALL TABLES IN SCHEMA public TO "%s";
	GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO "%s";
	ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public
	GRANT ALL ON TABLES TO "%s";
	ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public
	GRANT ALL ON SEQUENCES TO "%s";
	ALTER DATABASE "%s" OWNER TO "%s";
	`, userName, userName, userName, userName, userName, userName, userName, dbName, userName))
	return err
}
