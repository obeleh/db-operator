package postgres

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/query_utils"
)

func NewDatabasePrivsReconciler(privs dboperatorv1alpha1.DbPriv, conn *sql.DB, userName string, dbName string, normalizedPrivSet []string, serverVersion *PostgresVersion) (*PrivsReconciler, error) {
	var privsGetterFn privsGetter
	if serverVersion.ProductName == CockroachDB {
		privsGetterFn = getDatabasePrivilegesCrdb
	} else {
		privsGetterFn = getDatabasePrivilegesPg
	}

	return &PrivsReconciler{
		DbPriv:         privs,
		DesiredPrivSet: normalizedPrivSet,
		UserName:       userName,
		scopedName:     dbName,
		conn:           conn,
		grantFun:       grantDatabasePrivileges,
		revokeFun:      revokeDatabasePrivileges,
		privsGetFun:    privsGetterFn,
	}, nil
}

func grantDatabasePrivileges(conn *sql.DB, user string, db string, privs []string) error {
	privsStr := strings.Join(privs, ", ")
	escapedDb := pq.QuoteIdentifier(db)
	escapedUser := pq.QuoteIdentifier(user)

	if user == "PUBLIC" {
		query := fmt.Sprintf("GRANT %s ON DATABASE %s TO PUBLIC", privsStr, escapedDb)
		_, err := conn.Exec(query)
		return err
	} else {
		query := fmt.Sprintf("GRANT %s ON DATABASE %s TO %s", privsStr, escapedDb, escapedUser)
		_, err := conn.Exec(query)
		return err
	}
}

func revokeDatabasePrivileges(conn *sql.DB, user string, db string, privs []string) error {
	privsStr := strings.Join(privs, ", ")
	escapedDb := pq.QuoteIdentifier(db)
	escapedUser := pq.QuoteIdentifier(user)

	if user == "PUBLIC" {
		query := fmt.Sprintf("REVOKE %s ON DATABASE %s FROM PUBLIC", privsStr, escapedDb)
		_, err := conn.Exec(query)
		return err
	} else {
		query := fmt.Sprintf("REVOKE %s ON DATABASE %s FROM %s", privsStr, escapedDb, escapedUser)
		_, err := conn.Exec(query)
		return err
	}
}

func getDatabasePrivilegesCrdb(conn *sql.DB, user string, db string) ([]string, error) {
	quotedDb := pq.QuoteIdentifier(db)
	quotedUser := pq.QuoteIdentifier(user)

	rows, err := conn.Query(fmt.Sprintf("SHOW GRANTS ON DATABASE %s FOR %s;", quotedDb, quotedUser))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	privileges := []string{}
	for rows.Next() {
		var databaseName, grantee, privilegeType string
		var isGrantable bool

		err = rows.Scan(&databaseName, &grantee, &privilegeType, &isGrantable)
		if err != nil {
			return privileges, err
		}
		privileges = append(privileges, privilegeType)
	}

	return privileges, nil
}

func getDatabasePrivilegesPg(conn *sql.DB, user string, db string) ([]string, error) {
	datacl, err := query_utils.SelectFirstValueStringNullToEmpty(conn, "SELECT datacl FROM pg_database WHERE datname = $1", db)
	if err != nil {
		return nil, fmt.Errorf("unable to read databasePrivs %s", err)
	}
	rePattern := fmt.Sprintf(`%s\\?"?=(C?T?c?)/[^,]+,?`, user)
	re := regexp.MustCompile(rePattern)
	returnArray := []string{}
	submatches := re.FindStringSubmatch(datacl)
	if len(submatches) > 0 {
		for _, chr := range submatches[1] {
			returnArray = append(returnArray, DATABASE_PRIV_MAP[string(chr)])
		}
	}

	serverVersion, err := getServerVersion(conn)
	if err != nil {
		return returnArray, err
	}

	return NormalizePrivileges(returnArray, "database", serverVersion), nil
}
