package mysql

// https://github.com/ansible-collections/community.mysql/blob/main/plugins/modules/mysql_user.py

import (
	path "path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/obeleh/db-operator/shared"
	v1 "k8s.io/api/core/v1"
)

type MySqlActions struct {
	shared.DbActionsBase
}

func (i *MySqlActions) BuildContainer(scriptName string) v1.Container {
	dbServer := i.DbServer
	envVars := []v1.EnvVar{
		{Name: "MYSQL_HOST", Value: dbServer.Spec.Address},
		{Name: "MYSQL_USER", Value: dbServer.Spec.UserName},
		{Name: "MYSQL_PWD", ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: dbServer.Spec.SecretName,
				},
				Key: shared.Nvl(dbServer.Spec.PasswordKey, "password"),
			},
		}},
		{Name: "MYSQL_DATABASE", Value: i.Db.Spec.DbName},
	}

	return v1.Container{
		Name:  "mysql-" + shared.ReplaceNonAllowedChars(strings.Replace(scriptName, ".sh", "", 1)),
		Image: "mysql:" + shared.Nvl(dbServer.Spec.Version, "latest"),
		Env:   envVars,
		Command: []string{
			path.Join("/", shared.SCRIPTS_VOLUME_NAME, scriptName),
		},
		VolumeMounts: shared.VOLUME_MOUNTS,
	}
}

func (i *MySqlActions) BuildBackupContainer() v1.Container {
	return i.BuildContainer(shared.BACKUP_MYSQL)
}

func (i *MySqlActions) BuildRestoreContainer() v1.Container {
	return i.BuildContainer(shared.RESTORE_MYSQL)
}
