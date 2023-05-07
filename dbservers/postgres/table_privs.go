package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
)

func NewTablePrivsReconciler(privs dboperatorv1alpha1.DbPriv, conn *sql.DB, userName string, tableName string, normalizedPrivSet []string, serverVersion *PostgresVersion) *PrivsReconciler {
	return &PrivsReconciler{
		DbPriv:         privs,
		DesiredPrivSet: normalizedPrivSet,
		UserName:       userName,
		scopedName:     tableName,
		conn:           conn,
		grantFun:       grantTablePrivileges,
		revokeFun:      revokeTablePrivileges,
		privsGetFun:    getTablePrivileges,
	}
}

func getTablePrivileges(conn *sql.DB, user string, table string) ([]string, error) {
	var schema string
	if strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 1)
		schema, table = parts[0], parts[1]
	} else {
		schema = "public"
	}

	query := "SELECT privilege_type FROM information_schema.role_table_grants WHERE grantee=$1 AND table_name=$2 AND table_schema=$3"
	rows, err := conn.Query(query, user, table, schema)
	if err != nil {
		return nil, fmt.Errorf("unable to read tablePrivs %s", err)
	}
	defer rows.Close()

	tablePrivs := []string{}
	for rows.Next() {
		var privType string
		err := rows.Scan(&privType)
		if err != nil {
			return nil, fmt.Errorf("unable to load privType")
		}
		tablePrivs = append(tablePrivs, privType)
	}
	return tablePrivs, nil
}

func grantTablePrivileges(conn *sql.DB, user string, table string, privs []string) error {
	_, err := conn.Exec(fmt.Sprintf("GRANT %s ON TABLE %q TO %q", strings.Join(privs, ", "), table, user))
	return err
}

func revokeTablePrivileges(conn *sql.DB, user string, table string, privs []string) error {
	_, err := conn.Exec(fmt.Sprintf("REVOKE %s ON TABLE %q FROM %q", strings.Join(privs, ", "), table, user))
	return err
}
