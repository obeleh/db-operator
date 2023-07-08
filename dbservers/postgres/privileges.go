package postgres

// https://github.com/ansible-collections/community.postgresql/blob/main/plugins/modules/postgresql_user.py

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/query_utils"
	"github.com/obeleh/db-operator/shared"
	funk "github.com/thoas/go-funk"
)

type PostgresVersion struct {
	ProductName string
	VersionStr  string
	Major       int
	Minor       int
	Patch       int
}

type ConnectionGetter func(*string, *string) (*sql.DB, error)
type privsAdjuster func(*sql.DB, string, string, []string) error

const PostgreSQL = "PostgreSQL"
const CockroachDB = "CockroachDB"

func (p *PostgresVersion) GetValidPrivs(privType string) []string {
	if p.ProductName == PostgreSQL {
		return map[string][]string{
			"database": {"CREATE", "CONNECT", "TEMPORARY", "TEMP", "ALL"},
			"table":    {"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "ALL"},
			"schema":   {"CREATE", "USAGE"}, // accepted by has_schema_privilege
		}[privType]
	} else if p.ProductName == CockroachDB {
		return map[string][]string{
			"database": {"CREATE", "CONNECT", "BACKUP", "RESTORE", "ALL"},
			"table":    {"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "BACKUP", "ALL"},
			"schema":   {"CREATE", "USAGE"}, // accepted by has_schema_privilege
		}[privType]
	}
	log.Fatalf("Unknown product name %s", p.ProductName)
	return []string{}
}

func StringNotEmpty(input string) bool {
	return len(input) != 0
}

func Remove(arr []string, item string) []string {
	output := []string{}
	for _, value := range arr {
		if value != item {
			output = append(output, value)
		}
	}
	return output
}

var FLAGS = []string{"SUPERUSER", "CREATEROLE", "CREATEDB", "INHERIT", "LOGIN", "REPLICATION"}
var PRIV_TO_AUTHID_COLUMN = map[string]string{
	"SUPERUSER":   "rolsuper",
	"CREATEROLE":  "rolcreaterole",
	"CREATEDB":    "rolcreatedb",
	"INHERIT":     "rolinherit",
	"LOGIN":       "rolcanlogin",
	"REPLICATION": "rolreplication",
	"BYPASSRLS":   "rolbypassrls",
}
var FLAGS_BY_VERSION = map[string]int{
	"BYPASSRLS": 90500,
}
var DATABASE_PRIV_MAP = map[string]string{
	"C": "CREATE",
	"T": "TEMPORARY",
	"c": "CONNECT",
}

func getFlagsForVersion(version int) []string {
	flags := FLAGS
	for flag, version_for_flag := range FLAGS_BY_VERSION {
		if version >= version_for_flag {
			flags = append(flags, flag)
		}
	}
	toAdd := []string{}
	for _, flag := range flags {
		toAdd = append(toAdd, fmt.Sprintf("NO%s", flag))
	}
	return append(flags, toAdd...)
}

func ParseRoleAttrs(role_attr_flags string, version int) ([]string, error) {
	flags := funk.Map(funk.FilterString(strings.Split(role_attr_flags, ","), StringNotEmpty), strings.ToUpper).([]string)
	validFlags := getFlagsForVersion(version)
	if !funk.Subset(flags, validFlags) {
		difference := funk.Subtract(flags, validFlags).([]string)
		return nil, fmt.Errorf("invalid role_attr_flags specified: %s", strings.Join(difference, ","))
	}

	return flags, nil
}

func NormalizePrivileges(privs []string, privType string, serverVersion *PostgresVersion) []string {
	newPrivs := privs
	if funk.Contains(newPrivs, "ALL") {
		newPrivs = append(newPrivs, serverVersion.GetValidPrivs(privType)...)
		newPrivs = Remove(newPrivs, "ALL")
	}
	if funk.Contains(newPrivs, "TEMP") {
		newPrivs = append(newPrivs, "TEMPORARY")
		newPrivs = Remove(newPrivs, "TEMP")
	}
	return funk.UniqString(newPrivs)
}

func toPrivSet(input string) []string {
	trimmedItems := funk.Map(strings.Split(input, ","), strings.TrimSpace).([]string)
	return funk.Map(funk.FilterString(trimmedItems, StringNotEmpty), strings.ToUpper).([]string)
}

func ParsePrivs(privs string, db string, serverVersion *PostgresVersion) (map[string]map[string][]string, error) {
	if len(privs) == 0 {
		return nil, nil
	}

	oPrivs := map[string]map[string][]string{
		"database": {},
		"table":    {},
		"schema":   {},
	}

	for _, token := range strings.Split(privs, "/") {
		var privType string
		var name string
		var privSet []string

		if strings.Contains(token, ":") {
			privType = "table"
			elements := strings.Split(token, ":")
			name = elements[0]
			privileges := elements[1]
			privSet = toPrivSet(privileges)
		} else if strings.Contains(db, ".") {
			privType = "schema"
			name = db
			privSet = toPrivSet(token)
		} else {
			privType = "database"
			name = db
			privSet = toPrivSet(token)
		}

		if !funk.Subset(privSet, serverVersion.GetValidPrivs(privType)) {
			invalidPrivs := strings.Join(funk.Subtract(privSet, serverVersion.GetValidPrivs(privType)).([]string), " ")
			return nil, fmt.Errorf("invalid privs specified for %s: %s", privType, invalidPrivs)
		}
		privSet = NormalizePrivileges(privSet, privType, serverVersion)
		oPrivs[privType][name] = privSet
	}
	return oPrivs, nil
}

func parseVersionString(versionResult string) (*PostgresVersion, error) {
	parts := strings.Split(versionResult, " ")
	productName := parts[0]
	var versionStr string
	var versionParts []string
	if productName == PostgreSQL {
		versionStr = parts[1]
		versionParts = strings.Split(versionStr, ".")
	} else if productName == CockroachDB {
		versionStr = parts[2]
		versionParts = strings.Split(versionStr[1:], ".")
	} else {
		return nil, fmt.Errorf("unexpected productname %s", productName)
	}

	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, fmt.Errorf("failed parsing major version")
	}
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return nil, fmt.Errorf("failed parsing minor version")
	}

	patch := -1
	if len(versionParts) > 2 {
		patch, err = strconv.Atoi(versionParts[2])
		if err != nil {
			return nil, fmt.Errorf("failed parsing patch version")
		}
	}

	return &PostgresVersion{
		ProductName: productName,
		VersionStr:  versionStr,
		Major:       major,
		Minor:       minor,
		Patch:       patch,
	}, nil
}

func getServerVersion(conn *sql.DB) (*PostgresVersion, error) {
	versionResult, err := query_utils.SelectFirstValueString(conn, "SELECT version();")
	if err != nil {
		return nil, err
	}

	return parseVersionString(versionResult)
}

func RevokeAllDbPrivs(conn *sql.DB, user string, dbPrivs []dboperatorv1alpha1.DbPriv, connectionGetter ConnectionGetter) error {
	serverVersion, err := getServerVersion(conn)
	if err != nil {
		return err
	}

	for _, dbPriv := range dbPrivs {
		reconciler, err := GetPrivsReconciler(user, dbPriv, serverVersion, connectionGetter)
		if err != nil {
			return err
		}
		if reconciler.IsDefaultPrivReconciler {
			reconcilerConn := reconciler.GetConn()
			schemas, err := GetSchemas(reconcilerConn)
			if err != nil {
				return err
			}
			for schema := range schemas {
				err := revokeDefaultedPrivs(reconcilerConn, reconciler.DefaultPrivObjectType, user, schema)
				if err != nil {
					return err
				}
			}
		}
		err = reconciler.RevokeAllPrivs()
		if err != nil {
			return err
		}
	}
	return nil
}

func UpdateUserPrivs(conn *sql.DB, userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv, connectionGetter ConnectionGetter) (bool, error) {
	// Server Privs
	maps, err := shared.SelectToArrayMap(conn, "SELECT * FROM pg_roles WHERE rolname = $1", userName)
	if err != nil {
		return false, err
	}
	changed := false
	currentRoleAttrs := maps[0]
	var serverPrivsChanging bool = false
	// TODO Scan server version and add to parser

	serverVersion, err := getServerVersion(conn)
	if err != nil {
		return false, err
	}

	roleAttrFlags, err := ParseRoleAttrs(serverPrivs, 0)
	if err != nil {
		return false, err
	}
	if len(roleAttrFlags) > 0 {
		for _, flag := range roleAttrFlags {
			roleAttrValue := !strings.HasPrefix(flag, "NO")
			if currentRoleAttrs[PRIV_TO_AUTHID_COLUMN[flag]] != roleAttrValue {
				serverPrivsChanging = true
			}
		}
	}

	if serverPrivsChanging {
		escapedUser := pq.QuoteIdentifier(userName)
		alter := []string{fmt.Sprintf("ALTER USER %s", escapedUser)}
		if len(roleAttrFlags) > 0 {
			alter = append(alter, fmt.Sprintf("WITH %s", strings.Join(roleAttrFlags, " ")))
		}

		_, err = conn.Exec(strings.Join(alter, " "))
		if err != nil {
			return false, err
		}
		changed = true
	}

	for _, dbPriv := range dbPrivs {
		privReconciler, err := GetPrivsReconciler(userName, dbPriv, serverVersion, connectionGetter)
		if err != nil {
			return changed, err
		}
		curChange, err := privReconciler.ReconcilePrivs()
		if err != nil {
			return changed, err
		}
		changed = changed || curChange
	}
	return changed, nil
}

func revokeDefaultedPrivs(conn *sql.DB, objType string, userName string, schemaName string) error {
	quotedUserName := pq.QuoteIdentifier(userName)
	quotedSchemaName := pq.QuoteIdentifier(schemaName)
	if !shared.IsAllowedVariable(objType, []string{"TABLES", "SEQUENCES", "FUNCTIONS"}, false) {
		return fmt.Errorf("invalid object type %s", objType)
	}
	_, err := conn.Exec(fmt.Sprintf("REVOKE ALL PRIVILEGES ON ALL %s IN SCHEMA %s FROM %s;", objType, quotedSchemaName, quotedUserName))
	return err
}

func GetSchemas(conn *sql.DB) (map[string]shared.DbSideSchema, error) {
	rows, err := conn.Query("SELECT nspname FROM pg_catalog.pg_namespace WHERE nspname NOT IN ('crdb_internal', 'information_schema', 'pg_catalog', 'pg_extension', 'pg_toast');")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schemas := make(map[string]shared.DbSideSchema)

	for rows.Next() {
		var schema shared.DbSideSchema
		err := rows.Scan(&schema.SchemaName)
		if err != nil {
			return nil, fmt.Errorf("unable to load PostgresDb")
		}
		schemas[schema.SchemaName] = schema
	}
	return schemas, nil
}

func GetDatabasesUserHasAccessTo(conn *sql.DB, userName string) ([]string, error) {
	rows, err := conn.Query(`
		SELECT
			d.datname AS database_name
		FROM
			pg_database d
		CROSS JOIN
			pg_roles r
		WHERE
			d.datname NOT IN ('template0', 'template1', 'postgres')
			AND r.rolname = $1
			AND has_database_privilege(r.rolname, d.datname, 'CONNECT');
	`, userName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	databases := []string{}

	for rows.Next() {
		var database string
		err := rows.Scan(&database)
		if err != nil {
			return nil, fmt.Errorf("unable to load PostgresDb")
		}
		databases = append(databases, database)
	}
	return databases, nil
}

func GetDbNameFromScopeName(scopeName string) string {
	if strings.Contains(scopeName, ".") {
		parts := strings.Split(scopeName, ".")
		return parts[0]
	}
	return scopeName
}

func GetScopeAfterDb(scopeName string) (string, error) {
	parts := strings.Split(scopeName, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("expected two parts in scope '%s' got %d", scopeName, len(parts))
	}
	return parts[1], nil
}
