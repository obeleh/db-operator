package shared

import (
	"database/sql"
	"fmt"
	"log"
	path "path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

type AlreadyHandledError struct {
	LoggedError error
}

func (ale AlreadyHandledError) Error() string {
	return ale.LoggedError.Error()
}

func NewAlreadyHandledError(err error) AlreadyHandledError {
	return AlreadyHandledError{
		LoggedError: err,
	}
}

func IsCannotFindError(err error) bool {
	statusErr, wasStatusErr := err.(*errors.StatusError)
	return statusErr != nil && wasStatusErr && statusErr.ErrStatus.Reason == "NotFound"
}

func CannotFindError(err error, log *zap.Logger, kind, namespace, name string) bool {
	if IsCannotFindError(err) {
		if kind != "" {
			log.Info(fmt.Sprintf("Object: %s.%s.%s not found", kind, namespace, name))
		} else {
			log.Info(fmt.Sprintf("Object: %s.%s not found", namespace, name))
		}
		return true
	}
	return false
}

func AlreadyExistsError(err error, log *zap.Logger, kind, namespace, name string) bool {
	statusErr, wasStatusErr := err.(*errors.StatusError)
	if statusErr != nil && wasStatusErr && statusErr.ErrStatus.Reason == "AlreadyExists" {
		log.Info(fmt.Sprintf("Object: %s.%s.%s not found", kind, namespace, name))
		return true
	}
	return false
}

func IsHandledErr(err error) bool {
	_, wasHandledErr := err.(AlreadyHandledError)
	return wasHandledErr
}

func GetTypeName(myvar interface{}) string {
	if t := reflect.TypeOf(myvar); t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	} else {
		return t.Name()
	}
}

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
	defer rows.Close()

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

func SelectToPropertyMap(conn *sql.DB, query string, key string, value string, args ...interface{}) (map[string]interface{}, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var keyCol int = -1
	var valCol int = -1
	for i, colName := range cols {
		if colName == key {
			keyCol = i
		}
		if colName == value {
			valCol = i
		}
	}
	if keyCol == -1 {
		return nil, fmt.Errorf("KeyColumn not found %s", key)
	}
	if valCol == -1 {
		return nil, fmt.Errorf("ValueColumn not found %s", value)
	}

	propertyMap := make(map[string]interface{}, 0)

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

		keyVal := columns[keyCol].(string)
		propertyMap[keyVal] = columns[valCol]
	}

	return propertyMap, nil
}

func Min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func Max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func GradualBackoffRetry(creationTime time.Time) ctrl.Result {
	// waits for 1 sec minimum and 300 sec max
	retrySecs := Max(Min(time.Since(creationTime).Seconds(), 300), 1)
	return RetryAfter(retrySecs)
}

func RetryAfter(secs float64) ctrl.Result {
	return ctrl.Result{
		// Gradual backoff
		Requeue:      true,
		RequeueAfter: time.Duration(secs) * time.Second,
	}
}

func IsAllowedVariable(name string, allowedOptions []string, caseSensitive bool) bool {
	for _, option := range allowedOptions {
		if caseSensitive {
			if name == option {
				return true
			}
		} else {
			if strings.ToLower(name) == strings.ToLower(option) {
				return true
			}
		}
	}
	return false
}
