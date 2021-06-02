package dbservers

import (
	"fmt"
	"strings"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	"github.com/kabisa/db-operator/dbservers/mysql"
	"github.com/kabisa/db-operator/dbservers/postgres"
	"github.com/kabisa/db-operator/shared"
	_ "github.com/lib/pq"
)

func GetServerActions(serverType string, dbServer *dboperatorv1alpha1.DbServer, db *dboperatorv1alpha1.Db, password string) (shared.DbActions, error) {
	var actions shared.DbActions

	if strings.ToLower(serverType) == "postgres" {
		pgActions := &postgres.PostgresDbInfo{
			DbInfo: shared.DbInfo{
				DbServer: dbServer,
				Db:       db,
				Password: password,
			},
		}
		actions = pgActions
	} else if strings.ToLower(serverType) == "mysql" {
		myActions := &mysql.MySqlDbInfo{
			DbInfo: shared.DbInfo{
				DbServer: dbServer,
				Db:       db,
				Password: password,
			},
		}
		actions = myActions
	} else {
		return nil, fmt.Errorf("expected either mysql or postgres server")
	}
	return actions, nil
}
