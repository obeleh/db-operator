package postgres

// https://github.com/ansible-collections/community.postgresql/blob/main/plugins/modules/postgresql_user.py

import (
	path "path/filepath"
	"strconv"
	"strings"

	"github.com/obeleh/db-operator/shared"
	v1 "k8s.io/api/core/v1"
)

type PostgresActions struct {
	shared.DbActionsBase
}

func (i *PostgresActions) BuildContainer(scriptName string) v1.Container {
	dbServer := i.DbServer
	envVars := []v1.EnvVar{
		{Name: "PGHOST", Value: dbServer.Spec.Address},
		{Name: "PGUSER", Value: dbServer.Spec.UserName},
		{Name: "PGPORT", Value: strconv.Itoa(dbServer.Spec.Port)},
		{Name: "PGPASSWORD", ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: dbServer.Spec.SecretName,
				},
				Key: shared.Nvl(dbServer.Spec.PasswordKey, "password"),
			},
		}},
		{Name: "DATABASE", Value: i.Db.Spec.DbName},
	}

	return v1.Container{
		Name:  "pg-" + shared.ReplaceNonAllowedChars(strings.Replace(scriptName, ".sh", "", 1)),
		Image: "postgres:" + shared.Nvl(dbServer.Spec.Version, "latest"),
		Env:   envVars,
		Command: []string{
			path.Join("/", shared.SCRIPTS_VOLUME_NAME, scriptName),
		},
		VolumeMounts: shared.VOLUME_MOUNTS,
	}
}

func (i *PostgresActions) BuildBackupContainer() v1.Container {
	return i.BuildContainer(shared.BACKUP_POSTGRES)
}

func (i *PostgresActions) BuildRestoreContainer() v1.Container {
	return i.BuildContainer(shared.RESTORE_POSTGRES)
}
