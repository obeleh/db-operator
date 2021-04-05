package controllers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-logr/logr"
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	_ "github.com/lib/pq"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDbConnection(log logr.Logger, k8sClient client.Client, ctx context.Context, dbServer *dboperatorv1alpha1.DbServer) (*sql.DB, error) {

	secretName := types.NamespacedName{
		Name:      dbServer.Spec.SecretName,
		Namespace: dbServer.Namespace,
	}
	secret := &v1.Secret{}

	err := k8sClient.Get(ctx, secretName, secret)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get secret: %s", dbServer.Spec.SecretName))
		return nil, err
	}

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbServer.Spec.Address, dbServer.Spec.Port, dbServer.Spec.UserName, secret.Data[dbServer.Spec.SecretKey], "postgres")
	pgDbServer, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to open a DB connection: %s", psqlInfo))
		return nil, err
	}
	return pgDbServer, nil
}

func GetDbConnectionFromUser(log logr.Logger, k8sClient client.Client, ctx context.Context, dbUser *dboperatorv1alpha1.User) (*sql.DB, error) {
	serverName := types.NamespacedName{
		Name:      dbUser.Spec.DbServerName,
		Namespace: dbUser.Namespace,
	}
	dbServer := &dboperatorv1alpha1.DbServer{}

	err := k8sClient.Get(ctx, serverName, dbServer)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get Server: %s", dbUser.Spec.DbServerName))
		return nil, err
	}

	return GetDbConnection(log, k8sClient, ctx, dbServer)
}

func GetDbs(log logr.Logger, dbServerConn *sql.DB) ([]string, error) {
	rows, err := dbServerConn.Query("SELECT datname FROM pg_database WHERE datistemplate = false;")
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to read databases from server"))
		return nil, err
	}

	databases := make([]string, 0)

	for rows.Next() {
		var databaseName string
		err := rows.Scan(&databaseName)
		if err != nil {
			break
		}
		databases = append(databases, databaseName)
	}
	return databases, nil
}

type PostgresUser struct {
	UserName   string
	Attributes string
}

func GetUsers(log logr.Logger, dbServerConn *sql.DB) (map[string]PostgresUser, error) {
	log.Info("Getting users")
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
		log.Error(err, fmt.Sprintf("Unable to read users from server"))
		return nil, err
	}

	postgresUsers := make(map[string]PostgresUser)

	for rows.Next() {
		var postgresUser PostgresUser
		err := rows.Scan(&postgresUser.UserName, &postgresUser.Attributes)
		if err != nil {
			log.Error(err, "unable to load postgresUser")
			break
		}
		log.Info(fmt.Sprintf("Found user %s", postgresUser.UserName))
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
