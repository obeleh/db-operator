package controllers

// https://github.com/ansible-collections/community.postgresql/blob/main/plugins/modules/postgresql_user.py

import (
	path "path/filepath"
	"strings"

	v1 "k8s.io/api/core/v1"
)

type PostgresDbInfo struct {
	DbInfo
}

func (i *PostgresDbInfo) GetDbConnection() (DbServerConnectionInterface, error) {
	var dbName string
	if i.Db == nil {
		dbName = "postgres"
	} else {
		dbName = i.Db.Spec.DbName
	}
	dbServer := i.DbServer
	conn := &PostgresConnection{
		DbServerConnection: DbServerConnection{
			DbServerConnectInfo: DbServerConnectInfo{
				Host:     dbServer.Spec.Address,
				Port:     dbServer.Spec.Port,
				UserName: dbServer.Spec.UserName,
				Password: i.Password,
				Database: dbName,
			},
			Driver: "postgres",
		},
	}
	conn.DbServerConnectionInterface = conn
	return conn, nil
}

func (i *PostgresDbInfo) buildContainer(scriptName string) v1.Container {
	dbServer := i.DbServer
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
		{Name: "DATABASE", Value: i.Db.Spec.DbName},
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

func (i *PostgresDbInfo) BuildBackupContainer() v1.Container {
	return i.buildContainer(BACKUP_POSTGRES)
}

func (i *PostgresDbInfo) BuildRestoreContainer() v1.Container {
	return i.buildContainer(RESTORE_POSTGRES)
}
