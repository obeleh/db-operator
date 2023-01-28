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

func GetServerActions(serverType string, dbServer *dboperatorv1alpha1.DbServer, db *dboperatorv1alpha1.Db, password string, options map[string]string) (shared.DbActions, error) {
	var actions shared.DbActions

	if strings.ToLower(serverType) == "postgres" {
		pgActions := &postgres.PostgresDbInfo{
			DbInfo: shared.DbInfo{
				DbServer: dbServer,
				Db:       db,
				Password: password,
				Options:  options,
			},
		}
		actions = pgActions
	} else if strings.ToLower(serverType) == "mysql" {
		myActions := &mysql.MySqlDbInfo{
			DbInfo: shared.DbInfo{
				DbServer: dbServer,
				Db:       db,
				Password: password,
				Options:  options,
			},
		}
		actions = myActions
	} else {
		return nil, fmt.Errorf("expected either mysql or postgres server")
	}
	return actions, nil
}
