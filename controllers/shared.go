package controllers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	path "path/filepath"
	"regexp"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	_ "github.com/lib/pq"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const SCRIPTS_CONFIGMAP string = "db-operator-scripts"

var VOLUME_MOUNTS = []v1.VolumeMount{
	{Name: SCRIPTS_VOLUME_NAME, MountPath: path.Join("/", SCRIPTS_VOLUME_NAME)},
	{Name: BACKUP_VOLUME_NAME, MountPath: path.Join("/", BACKUP_VOLUME_NAME)},
}

func GetVolumes() []v1.Volume {
	var defaultMode = new(int32)
	*defaultMode = 511 //  0777

	return []v1.Volume{
		{
			Name: SCRIPTS_VOLUME_NAME,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: SCRIPTS_CONFIGMAP,
					},
					Items:       []v1.KeyToPath{},
					DefaultMode: defaultMode,
				},
			},
		},
		{
			Name: BACKUP_VOLUME_NAME,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}
}

func GetUserPassword(dbUser *dboperatorv1alpha1.User, k8sClient client.Client, ctx context.Context) (*string, error) {
	secretName := types.NamespacedName{
		Name:      dbUser.Spec.SecretName,
		Namespace: dbUser.Namespace,
	}
	secret := &v1.Secret{}
	err := k8sClient.Get(ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %s", dbUser.Spec.SecretName)
	}

	passBytes, ok := secret.Data[Nvl(dbUser.Spec.SecretKey, "password")]
	if !ok {
		return nil, fmt.Errorf("password key (%s) not found in secret", Nvl(dbUser.Spec.SecretKey, "password"))
	}

	password := string(passBytes)

	return &password, nil
}

func Nvl(val1 string, val2 string) string {
	if len(val1) == 0 {
		return val2
	} else {
		return val1
	}
}

func ReplaceNonAllowedChars(input string) string {
	reg, err := regexp.Compile("[^A-Za-z0-9]+")
	if err != nil {
		log.Fatal(err)
	}
	return reg.ReplaceAllString(input, "-")
}

func SelectToArrayMap(conn *sql.DB, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	rowMaps := make([]map[string]interface{}, 0)

	for rows.Next() {
		// Create a slice of interface{}'s to represent each column,
		// and a second slice to contain pointers to each item in the columns slice.
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		// Scan the result into the column pointers...
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		// Create our map, and retrieve the value for each column from the pointers slice,
		// storing it in the map with the name of the column as the key.
		m := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			m[colName] = *val
		}
		rowMaps = append(rowMaps, m)
	}

	return rowMaps, nil
}
