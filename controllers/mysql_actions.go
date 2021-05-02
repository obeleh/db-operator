package controllers

// https://github.com/ansible-collections/community.mysql/blob/main/plugins/modules/mysql_user.py

import (
	path "path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	v1 "k8s.io/api/core/v1"
)

type MySqlDbActions struct{}

func (a *MySqlDbActions) GetDbConnection(dbInfo *DbInfo) (DbServerConnectionInterface, error) {
	var dbName string
	if dbInfo.Db == nil {
		dbName = ""
	} else {
		dbName = *&dbInfo.Db.Spec.DbName
	}
	dbServer := dbInfo.DbServer
	conn := &MySqlConnection{
		DbServerConnection: DbServerConnection{
			DbServerConnectInfo: DbServerConnectInfo{
				Host:     dbServer.Spec.Address,
				Port:     dbServer.Spec.Port,
				UserName: dbServer.Spec.UserName,
				Password: dbInfo.Password,
				Database: dbName,
			},
			Driver: "mysql",
		},
	}
	conn.DbServerConnectionInterface = conn
	return conn, nil
}

func (a *MySqlDbActions) buildContainer(dbInfo *DbInfo, scriptName string) v1.Container {
	dbServer := dbInfo.DbServer
	envVars := []v1.EnvVar{
		{Name: "MYSQL_HOST", Value: dbServer.Spec.Address},
		{Name: "MYSQL_USER", Value: dbServer.Spec.UserName},
		{Name: "MYSQL_PASSWORD", ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: dbServer.Spec.SecretName,
				},
				Key: Nvl(dbServer.Spec.SecretKey, "password"),
			},
		}},
		{Name: "MYSQL_DATABASE", Value: dbInfo.Db.Spec.DbName},
	}

	return v1.Container{
		Name:  "mysql-" + ReplaceNonAllowedChars(strings.Replace(scriptName, ".sh", "", 1)),
		Image: "mysql:" + Nvl(dbServer.Spec.Version, "latest"),
		Env:   envVars,
		Command: []string{
			path.Join("/", SCRIPTS_VOLUME_NAME, scriptName),
		},
		VolumeMounts: VOLUME_MOUNTS,
	}
}

func (a *MySqlDbActions) BuildBackupContainer(dbInfo *DbInfo) v1.Container {
	return a.buildContainer(dbInfo, BACKUP_MYSQL)
}

func (a *MySqlDbActions) BuildRestoreContainer(dbInfo *DbInfo) v1.Container {
	return a.buildContainer(dbInfo, RESTORE_MYSQL)
}
