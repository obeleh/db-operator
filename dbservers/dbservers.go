package dbservers

import (
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/mysql"
	"github.com/obeleh/db-operator/dbservers/postgres"
	"github.com/obeleh/db-operator/shared"
)

func GetServerActions(dbServer *dboperatorv1alpha1.DbServer, db *dboperatorv1alpha1.Db, credentials shared.Credentials, options map[string]string) (shared.DbActions, error) {
	var actions shared.DbActions

	serverType := dbServer.Spec.ServerType
	if strings.ToLower(serverType) == "postgres" || strings.ToLower(serverType) == "cockroachdb" {
		pgActions := &postgres.PostgresDbInfo{
			DbInfo: shared.DbInfo{
				Credentials: credentials,
				DbServer:    dbServer,
				Db:          db,
				Options:     options,
			},
		}
		actions = pgActions
	} else if strings.ToLower(serverType) == "mysql" {
		myActions := &mysql.MySqlDbInfo{
			DbInfo: shared.DbInfo{
				Credentials: credentials,
				DbServer:    dbServer,
				Db:          db,
				Options:     options,
			},
		}
		actions = myActions
	} else {
		return nil, fmt.Errorf("expected either mysql or postgres server")
	}
	return actions, nil
}
