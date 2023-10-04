package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/query_utils"
)

func NewSchemaPrivsReconciler(privs dboperatorv1alpha1.DbPriv, conn *sql.DB, userName string, schemaName string, normalizedPrivSet []string, serverVersion *PostgresVersion) (*PrivsReconciler, error) {
	return &PrivsReconciler{
		DbPriv:         privs,
		DesiredPrivSet: normalizedPrivSet,
		UserName:       userName,
		scopedName:     schemaName,
		conn:           conn,
		grantFun:       grantSchemaPrivileges,
		revokeFun:      revokeSchemaPrivileges,
		privsGetFun:    getSchemaPrivileges,
	}, nil
}

func getSchemaPrivileges(conn *sql.DB, user string, schema string) ([]string, error) {
	if strings.Contains(schema, ".") {
		return nil, fmt.Errorf("expected no dot in schema at this point")
	}
	createPriv, err := query_utils.SelectFirstValueBool(conn, "SELECT pg_catalog.has_schema_privilege($1, $2, 'CREATE')", user, schema)
	if err != nil {
		return nil, fmt.Errorf("unable to read schemaPrivs %s", err)
	}

	usagePriv, err := query_utils.SelectFirstValueBool(conn, "SELECT pg_catalog.has_schema_privilege($1, $2, 'USAGE')", user, schema)
	if err != nil {
		return nil, fmt.Errorf("unable to read schemaPrivs %s", err)
	}

	schemaPrivs := []string{}
	if createPriv {
		schemaPrivs = append(schemaPrivs, "CREATE")
	}

	if usagePriv {
		schemaPrivs = append(schemaPrivs, "USAGE")
	}

	return schemaPrivs, nil
}

func grantSchemaPrivileges(conn *sql.DB, user string, schemaName string, privs []string) error {
	privsStr := strings.Join(privs, ", ")
	escapedSchema := pq.QuoteIdentifier(schemaName)
	escapedUser := pq.QuoteIdentifier(user)

	query := fmt.Sprintf("GRANT %s on SCHEMA %s to %s;", privsStr, escapedSchema, escapedUser)
	_, err := conn.Exec(query) // nosemgrep, sql query is constructed from sanitized strings
	return err
}

func revokeSchemaPrivileges(conn *sql.DB, user string, schemaName string, privs []string) error {
	privsStr := strings.Join(privs, ", ")
	escapedSchema := pq.QuoteIdentifier(schemaName)
	escapedUser := pq.QuoteIdentifier(user)

	query := fmt.Sprintf("REVOKE %s on SCHEMA %s FROM %s;", privsStr, escapedSchema, escapedUser)
	_, err := conn.Exec(query) // nosemgrep, sql query is constructed from sanitized strings
	return err
}
