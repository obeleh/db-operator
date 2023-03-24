package dbservers

import (
	"fmt"
	"strings"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/mysql"
	"github.com/obeleh/db-operator/dbservers/postgres"
	"github.com/obeleh/db-operator/shared"
)

func GetServerActions(dbServer *dboperatorv1alpha1.DbServer, db *dboperatorv1alpha1.Db, options map[string]string) (shared.DbActions, error) {
	serverType := dbServer.Spec.ServerType
	if strings.ToLower(serverType) == "postgres" || strings.ToLower(serverType) == "cockroachdb" {
		return &postgres.PostgresActions{
			DbActionsBase: shared.DbActionsBase{
				DbServer: dbServer,
				Db:       db,
				Options:  options,
			},
		}, nil
	} else if strings.ToLower(serverType) == "mysql" {
		return &mysql.MySqlActions{
			DbActionsBase: shared.DbActionsBase{
				DbServer: dbServer,
				Db:       db,
				Options:  options,
			},
		}, nil
	} else {
		return nil, fmt.Errorf("expected either mysql or postgres server")
	}
}
