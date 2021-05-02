package controllers

// https://github.com/ansible-collections/community.postgresql/blob/main/plugins/modules/postgresql_user.py

import (
	path "path/filepath"
	"strings"

	v1 "k8s.io/api/core/v1"
)

type PostgresDbActions struct{}

func (a *PostgresDbActions) GetDbConnection(dbInfo *DbInfo) (DbServerConnectionInterface, error) {
	var dbName string
	if dbInfo.Db == nil {
		dbName = "postgres"
	} else {
		dbName = dbInfo.Db.Spec.DbName
	}
	dbServer := dbInfo.DbServer
	conn := &PostgresConnection{
		DbServerConnection: DbServerConnection{
			DbServerConnectInfo: DbServerConnectInfo{
				Host:     dbServer.Spec.Address,
				Port:     dbServer.Spec.Port,
				UserName: dbServer.Spec.UserName,
				Password: dbInfo.Password,
				Database: dbName,
			},
			Driver: "postgres",
		},
	}
	conn.DbServerConnectionInterface = conn
	return conn, nil
}

func (a *PostgresDbActions) buildContainer(dbInfo *DbInfo, scriptName string) v1.Container {
	dbServer := dbInfo.DbServer
	envVars := []v1.EnvVar{
		{Name: "PGHOST", Value: dbServer.Spec.Address},
		{Name: "PGUSER", Value: dbServer.Spec.UserName},
		{Name: "PGPASSWORD", ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: dbServer.Spec.SecretName,
				},
				Key: Nvl(dbServer.Spec.SecretKey, "password"),
			},
		}},
		{Name: "DATABASE", Value: dbInfo.Db.Spec.DbName},
	}

	return v1.Container{
		Name:  "pg-" + ReplaceNonAllowedChars(strings.Replace(scriptName, ".sh", "", 1)),
		Image: "postgres:" + Nvl(dbServer.Spec.Version, "latest"),
		Env:   envVars,
		Command: []string{
			path.Join("/", SCRIPTS_VOLUME_NAME, scriptName),
		},
		VolumeMounts: VOLUME_MOUNTS,
	}
}

func (a *PostgresDbActions) BuildBackupContainer(dbInfo *DbInfo) v1.Container {
	return a.buildContainer(dbInfo, BACKUP_POSTGRES)
}

func (a *PostgresDbActions) BuildRestoreContainer(dbInfo *DbInfo) v1.Container {
	return a.buildContainer(dbInfo, RESTORE_POSTGRES)
}
