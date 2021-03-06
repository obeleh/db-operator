package shared

import (
	"database/sql"
	"log"
	path "path/filepath"
	"regexp"

	v1 "k8s.io/api/core/v1"
)

const SCRIPTS_CONFIGMAP string = "db-operator-scripts"

func Nvl(val1 string, val2 string) string {
	if len(val1) == 0 {
		return val2
	} else {
		return val1
	}
}

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
