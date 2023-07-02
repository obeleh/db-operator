package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
)

func NewTablePrivsReconciler(privs dboperatorv1alpha1.DbPriv, conn *sql.DB, userName string, scopedName string, normalizedPrivSet []string, serverVersion *PostgresVersion) (*PrivsReconciler, error) {
	if strings.HasSuffix(scopedName, ".ALL") {
		scopedName = strings.TrimSuffix(scopedName, ".ALL")
		if !strings.Contains(scopedName, ".") {
			return nil, fmt.Errorf("invalid scope %q", scopedName)
		}
		parts := strings.SplitN(scopedName, ".", 2)
		schema := parts[1]

		return &PrivsReconciler{
			DbPriv:         privs,
			DesiredPrivSet: normalizedPrivSet,
			UserName:       userName,
			scopedName:     schema,
			conn:           conn,
			grantFun:       grantPrivilegesOnAllTables,
			revokeFun:      revokePrivilegesOnAllTables,
			privsGetFun: func(conn *sql.DB, user, scopedName string) ([]string, error) {
				return getTablePrivilegesForAllTables(conn, user, scopedName, normalizedPrivSet)
			},
		}, nil
	} else {
		return &PrivsReconciler{
			DbPriv:         privs,
			DesiredPrivSet: normalizedPrivSet,
			UserName:       userName,
			scopedName:     scopedName,
			conn:           conn,
			grantFun:       grantTablePrivileges,
			revokeFun:      revokeTablePrivileges,
			privsGetFun:    getTablePrivileges,
		}, nil
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
	privSet := strings.Join(privs, ", ")
	quotedTableName := pq.QuoteIdentifier(table)
	quotedUserName := pq.QuoteIdentifier(user)
	_, err := conn.Exec(fmt.Sprintf("GRANT %s ON TABLE %s TO %s", privSet, quotedTableName, quotedUserName)) // nosemgrep, sql query is constructed from sanitized strings
	return err
}

func revokeTablePrivileges(conn *sql.DB, user string, table string, privs []string) error {
	privSet := strings.Join(privs, ", ")
	quotedTableName := pq.QuoteIdentifier(table)
	quotedUserName := pq.QuoteIdentifier(user)
	_, err := conn.Exec(fmt.Sprintf("REVOKE %s ON TABLE %s FROM %s", privSet, quotedTableName, quotedUserName)) // nosemgrep, sql query is constructed from sanitized strings
	return err
}

func getTablePrivilegesForAllTables(conn *sql.DB, user string, schema string, privs []string) ([]string, error) {
	privsFound := []string{}

	for _, priv := range privs {
		// Check if we can find a table that the user does not have the privilege on
		query := `SELECT table_schema, table_name 
		FROM information_schema.tables 
		WHERE table_schema = $2 
		AND has_table_privilege($1, table_schema || '.' || table_name, $3) = false;
		`
		rows, err := conn.Query(query, user, schema, strings.ToUpper(priv)) // nosemgrep, sql query is constructed from sanitized strings
		if err != nil {
			return nil, fmt.Errorf("unable to read tablePrivs %s", err)
		}
		// if we can't find a table, then the user has the privilege on all tables
		if !rows.Next() {
			privsFound = append(privsFound, priv)
		}
		rows.Close()
	}
	return privsFound, nil
}

func grantPrivilegesOnAllTables(conn *sql.DB, user string, schema string, privs []string) error {
	privSet := strings.Join(privs, ", ")
	quotedSchemaName := pq.QuoteIdentifier(schema)
	quotedUserName := pq.QuoteIdentifier(user)
	_, err := conn.Exec(fmt.Sprintf("GRANT %s ON ALL TABLES IN SCHEMA %s TO %q", privSet, quotedSchemaName, quotedUserName)) // nosemgrep, sql query is constructed from sanitized strings
	return err
}

func revokePrivilegesOnAllTables(conn *sql.DB, user string, schema string, privs []string) error {
	privSet := strings.Join(privs, ", ")
	quotedSchemaName := pq.QuoteIdentifier(schema)
	quotedUserName := pq.QuoteIdentifier(user)
	_, err := conn.Exec(fmt.Sprintf("REVOKE %s ON ALL TABLES IN SCHEMA %s FROM %q", privSet, quotedSchemaName, quotedUserName)) // nosemgrep, sql query is constructed from sanitized strings
	return err
}
