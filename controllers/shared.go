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

func GetUsers(log logr.Logger, dbServerConn *sql.DB) ([]PostgresUser, error) {
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

	postgresUsers := make([]PostgresUser, 0)

	for rows.Next() {
		var postgresUser PostgresUser
		err := rows.Scan(&postgresUser)
		if err != nil {
			break
		}
		postgresUsers = append(postgresUsers, postgresUser)
	}
	return postgresUsers, nil
}
