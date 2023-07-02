package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
)

func NewDefaultTablePrivsReconciler(privs dboperatorv1alpha1.DbPriv, conn *sql.DB, userName string, tableName string, normalizedPrivSet []string, serverVersion *PostgresVersion) (*PrivsReconciler, error) {
	objectType := "r" // r = relation (table, view)
	getDefaultTablePrivileges := func(conn *sql.DB, user string, scopedName string) ([]string, error) {
		return getDefaultPrivilegesGivenByCurrentUser(conn, objectType, user)
	}

	return &PrivsReconciler{
		DbPriv:                 privs,
		DesiredPrivSet:         normalizedPrivSet,
		UserName:               userName,
		scopedName:             tableName,
		conn:                   conn,
		grantFun:               grantDefaultTablePrivileges,
		revokeFun:              revokeDefaultTablePrivileges,
		privsGetFun:            getDefaultTablePrivileges,
		IsDefaultPrivRconciler: true,
	}, nil
}

func revokeDefaultTablePrivileges(conn *sql.DB, user string, role string, privs []string) error {
	privsStr := strings.Join(privs, ", ")
	escapedUser := pq.QuoteIdentifier(user)
	escapedRole := pq.QuoteIdentifier(role)

	query := fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s REVOKE %s ON TABLES FROM %s;", escapedRole, privsStr, escapedUser)
	_, err := conn.Exec(query) // nosemgrep, sql query is constructed from sanitized strings
	return err
}

func grantDefaultTablePrivileges(conn *sql.DB, user string, role string, privs []string) error {
	privsStr := strings.Join(privs, ", ")
	escapedUser := pq.QuoteIdentifier(user)
	escapedRole := pq.QuoteIdentifier(role)

	query := fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT %s ON TABLES TO %s;", escapedRole, privsStr, escapedUser)
	_, err := conn.Exec(query) // nosemgrep, sql query is constructed from sanitized strings
	return err
}

func getDefaultPrivilegesGivenByCurrentUser(conn *sql.DB, objectType string, user string) ([]string, error) {
	//https://www.dba-ninja.com/2020/06/how-to-find-default-access-privileges-on-postgresql-with-pg_default_acl.html
	/*
		acl:
		r–SELECT (read)
		a–INSERT (append)
		w–UPDATE (write)
		d–DELETE

		objectType:
		r = relation (table, view),
		S = sequence,
		f = function,
		T = type,
		n = schema
	*/

	query := `SELECT d.defaclacl
	FROM pg_catalog.pg_default_acl d left join pg_catalog.pg_namespace n on n.oid = d.defaclnamespace
	WHERE pg_get_userbyid(d.defaclrole) = current_user
	AND n.nspname is NULL
	AND d.defaclobjtype = $1;
	`
	rows, err := conn.Query(query, objectType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	privileges := []string{}
	if rows.Next() {
		var defaclacls []string

		// .Scan(
		err := rows.Scan(pq.Array(&defaclacls))
		if err != nil {
			return nil, err
		}

		for _, defaclacl := range defaclacls {
			parts := strings.Split(defaclacl, "=")
			grantee := parts[0]
			grantee = stripOuterChars(grantee, "\"")
			privs := parts[1]
			if strings.Contains(privs, "/") {
				privs = strings.Split(privs, "/")[0]
			}

			if objectType == "r" { // r = relation (table, view),
				if user == grantee {
					if strings.Contains(privs, "a") {
						privileges = append(privileges, "INSERT")
					}
					if strings.Contains(privs, "r") {
						privileges = append(privileges, "SELECT")
					}
					if strings.Contains(privs, "w") {
						privileges = append(privileges, "UPDATE")
					}
					if strings.Contains(privs, "d") {
						privileges = append(privileges, "DELETE")
					}
				}
			} else {
				return nil, fmt.Errorf("currently only default privileges for r(elation) is supported")
			}
		}

		if rows.Next() {
			return nil, fmt.Errorf("unexpected amount of rows returned when running getDefaultPrivileges")
		}
	}
	return privileges, nil
}

func stripOuterChars(objectName string, chars string) string {
	if strings.HasPrefix(objectName, chars) && strings.HasSuffix(objectName, chars) {
		objectName = strings.TrimPrefix(objectName, chars)
		objectName = strings.TrimSuffix(objectName, chars)
	}
	return objectName
}
