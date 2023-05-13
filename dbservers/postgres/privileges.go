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
var ADJUST_PRIVILEGES_MAP = map[string]map[string]privsAdjuster{
	"table": {
		"revoke": revokeTablePrivileges,
		"grant":  grantTablePrivileges,
	},
	"defaultTable": {
		"revoke": revokeDefaultTablePrivileges,
		"grant":  grantDefaultTablePrivileges,
	},
	"database": {
		"revoke": revokeDatabasePrivileges,
		"grant":  grantDatabasePrivileges,
	},
	"schema": {
		"revoke": revokeSchemaPrivileges,
		"grant":  grantSchemaPrivileges,
	},
}
var CHECK_PRIVILEGES_MAP = map[string]func(*sql.DB, string, string, []string, *PostgresVersion) ([]string, []string, []string, error){
	"table":        hasTablePrivileges,
	"database":     hasDatabasePrivileges,
	"schema":       hasSchemaPrivileges,
	"defaultTable": hasDefaultTablePrivileges,
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
		return nil, fmt.Errorf("Unexpected productname %s", productName)
	}

	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, fmt.Errorf("Failed parsing major version")
	}
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return nil, fmt.Errorf("Failed parsing minor version")
	}

	patch := -1
	if len(versionParts) > 2 {
		patch, err = strconv.Atoi(versionParts[2])
		if err != nil {
			return nil, fmt.Errorf("Failed parsing patch version")
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

func revokeAllOfPrivMap(conn *sql.DB, privsMapMap map[string]map[string][]string, user string) error {
	for privType, privsMap := range privsMapMap {
		for name, privs := range privsMap {
			revokeFun := ADJUST_PRIVILEGES_MAP[privType]["revoke"]
			err := revokeFun(conn, user, name, privs)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RevokeAllDbPrivs(conn *sql.DB, user string, dbPrivs []dboperatorv1alpha1.DbPriv, connectionGetter ConnectionGetter) error {
	serverVersion, err := getServerVersion(conn)
	if err != nil {
		return err
	}

	for _, dbPriv := range dbPrivs {

		dbName := GetDbNameFromScopeName(dbPriv.Scope)
		conn, err := connectionGetter(dbPriv.Grantor, &dbName)
		if err != nil {
			return err
		}

		if dbPriv.DefaultPrivs != "" {
			// get schema's this user could get default privs on from current grantor connection
			schemas, err := GetSchemas(conn)
			if err != nil {
				return err
			}
			objType, err := GetScopeAfterDb(dbPriv.Scope)
			if err != nil {
				return err
			}
			for schema := range schemas {
				err := revokeDefaultedPrivs(conn, objType, user, schema)
				if err != nil {
					return err
				}
			}

			privsMapMap, err := ParseDefaultPrivs(dbPriv.DefaultPrivs, dbPriv.Scope, *dbPriv.Grantor, serverVersion)
			if err != nil {
				return err
			}
			err = revokeAllOfPrivMap(conn, privsMapMap, user)
			if err != nil {
				return err
			}
		}

		if dbPriv.Privs != "" {
			privsMapMap, err := ParsePrivs(dbPriv.Privs, dbPriv.Scope, serverVersion)
			if err != nil {
				return err
			}
			err = revokeAllOfPrivMap(conn, privsMapMap, user)
			if err != nil {
				return err
			}
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

func updateDefaultPrivs(conn *sql.DB, userName string, dbPriv dboperatorv1alpha1.DbPriv, serverVersion *PostgresVersion) (bool, error) {
	if dbPriv.Grantor == nil {
		return false, fmt.Errorf("grantor needs to be filled in for default privileges")
	}
	privMap, err := ParseDefaultPrivs(dbPriv.DefaultPrivs, dbPriv.Scope, *dbPriv.Grantor, serverVersion)
	if err != nil {
		return false, err
	}
	return adjustPrivileges(conn, userName, privMap, serverVersion)
}

func ParseDefaultPrivs(defaultPrivs, scope, grantor string, serverVersion *PostgresVersion) (map[string]map[string][]string, error) {
	scopeAfterDb, err := GetScopeAfterDb(scope)
	if err != nil {
		return nil, err
	}

	if len(defaultPrivs) == 0 {
		return nil, nil
	}

	oPrivs := map[string]map[string][]string{
		"defaultTable": {},
	}

	var desiredPrivSet []string
	if scopeAfterDb == "TABLES" {
		name := grantor
		oPrivs["defaultTable"][name] = toPrivSet(defaultPrivs)
	} else {
		return nil, fmt.Errorf(fmt.Sprintf("Not implemented to update default privileges on %s", scope))
	}

	if !funk.Subset(desiredPrivSet, serverVersion.GetValidPrivs("table")) {
		invalidPrivs := strings.Join(funk.Subtract(desiredPrivSet, serverVersion.GetValidPrivs("table")).([]string), " ")
		return nil, fmt.Errorf("invalid privs specified for %s: %s", "table", invalidPrivs)
	}
	return oPrivs, nil
}

func updateDbPrivs(conn *sql.DB, userName string, dbPriv dboperatorv1alpha1.DbPriv, serverVersion *PostgresVersion) (bool, error) {
	privMap, err := ParsePrivs(dbPriv.Privs, dbPriv.Scope, serverVersion)
	if err != nil {
		return false, err
	}
	return adjustPrivileges(conn, userName, privMap, serverVersion)
}

/*
Return the difference between the privileges that a user already has and

	the privileges that they desire to have.
	:returns: tuple of:
	    * privileges that they have and were requested
	    * privileges they currently hold but were not requested
	    * privileges requested that they do not hold
*/
func hasTablePrivileges(conn *sql.DB, user string, table string, privs []string, serverVersion *PostgresVersion) ([]string, []string, []string, error) {
	curPrivs, err := getTablePrivileges(conn, user, table)
	return diffPrivs(curPrivs, privs, err)
}

func diffPrivs(curPrivs []string, privs []string, err error) ([]string, []string, []string, error) {
	if err != nil {
		return nil, nil, nil, err
	}
	haveCurrently := funk.Join(curPrivs, privs, funk.InnerJoin).([]string)
	otherCurrent, desired := funk.Difference(curPrivs, privs)
	return haveCurrently, otherCurrent.([]string), desired.([]string), err
}

func revokeDefaultedPrivs(conn *sql.DB, objType string, userName string, schemaName string) error {
	quotedUserName := pq.QuoteIdentifier(userName)
	quotedSchemaName := pq.QuoteIdentifier(schemaName)
	_, err := conn.Exec(fmt.Sprintf("REVOKE ALL PRIVILEGES ON ALL %s IN SCHEMA %s FROM %s;", objType, quotedSchemaName, quotedUserName))
	return err
}

/*
Return the difference between the privileges that a user already has and
the privileges that they desire to have.
:returns: tuple of:
  - privileges that they have and were requested
  - privileges they currently hold but were not requested
  - privileges requested that they do not hold
*/
func hasDatabasePrivileges(conn *sql.DB, user string, db string, privs []string, serverVersion *PostgresVersion) ([]string, []string, []string, error) {
	if serverVersion.ProductName == CockroachDB {
		curPrivs, err := getDatabasePrivilegesCrdb(conn, user, db)
		return diffPrivs(curPrivs, privs, err)
	} else {
		curPrivs, err := getDatabasePrivilegesPg(conn, user, db)
		return diffPrivs(curPrivs, privs, err)
	}
}

func hasSchemaPrivileges(conn *sql.DB, user string, schema string, privs []string, serverVersion *PostgresVersion) ([]string, []string, []string, error) {
	schemaName, err := GetScopeAfterDb(schema)
	if err != nil {
		return nil, nil, nil, err
	}
	curPrivs, err := getSchemaPrivileges(conn, user, schemaName)
	return diffPrivs(curPrivs, privs, err)
}

func adjustPrivileges(conn *sql.DB, user string, privsMapMap map[string]map[string][]string, serverVersion *PostgresVersion) (bool, error) {
	if len(privsMapMap) == 0 {
		return false, nil
	}
	changed := false
	errors := []error{}
	for privType, privsMap := range privsMapMap {
		for name, privs := range privsMap {
			checkFun := CHECK_PRIVILEGES_MAP[privType]
			_, otherCurrent, desired, err := checkFun(conn, user, name, privs, serverVersion)
			if err != nil {
				return false, err
			}
			if len(otherCurrent) > 0 {
				revokeFun := ADJUST_PRIVILEGES_MAP[privType]["revoke"]
				err := revokeFun(conn, user, name, otherCurrent)
				if err != nil {
					errors = append(errors, err)
				} else {
					changed = true
				}
			}
			if len(desired) > 0 {
				grantFun := ADJUST_PRIVILEGES_MAP[privType]["grant"]
				err := grantFun(conn, user, name, desired)
				if err != nil {
					errors = append(errors, err)
				} else {
					changed = true
				}
			}
		}
	}
	var errsErr error
	if len(errors) > 0 {
		errsErr = fmt.Errorf("adjustPrivileges had errors %s", errors)
	} else {
		errsErr = nil
	}
	return changed, errsErr
}

type DefualtPrivilege struct {
	PrivObjectType string
	ObjName        string
	Grantee        string
}

func hasDefaultTablePrivileges(conn *sql.DB, user string, database string, privs []string, serverVersion *PostgresVersion) ([]string, []string, []string, error) {
	objectType := "r" // r = relation (table, view),
	curPrivs, err := getDefaultPrivilegesGivenByCurrentUser(conn, objectType, user)
	return diffPrivs(curPrivs, privs, err)
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

func stripOuterChars(objectName string, chars string) string {
	if strings.HasPrefix(objectName, chars) && strings.HasSuffix(objectName, chars) {
		objectName = strings.TrimPrefix(objectName, chars)
		objectName = strings.TrimSuffix(objectName, chars)
	}
	return objectName
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
		return "", fmt.Errorf("Expected two parts in scope '%s' got %d", scopeName, len(parts))
	}
	return parts[1], nil
}

/*
TABLE PRIVS:
SELECT grantee AS user, CONCAT(table_schema, '.', table_name) AS table,
    CASE
        WHEN COUNT(privilege_type) = 7 THEN 'ALL'
        ELSE ARRAY_TO_STRING(ARRAY_AGG(privilege_type), ', ')
    END AS grants
FROM information_schema.role_table_grants
GROUP BY table_name, table_schema, grantee;

ROLES:
SELECT * FROM pg_roles;

OR:
SELECT r.rolname, r.rolsuper, r.rolinherit,
  r.rolcreaterole, r.rolcreatedb, r.rolcanlogin,
  r.rolconnlimit, r.rolvaliduntil,
  ARRAY(SELECT b.rolname
        FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles b ON (m.roleid = b.oid)
        WHERE m.member = r.oid) as memberof
, r.rolreplication
, r.rolbypassrls
FROM pg_catalog.pg_roles r
WHERE r.rolname !~ '^pg_'
ORDER BY 1;



*/
