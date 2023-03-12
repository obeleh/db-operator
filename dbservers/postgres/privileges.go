package postgres

// https://github.com/ansible-collections/community.postgresql/blob/main/plugins/modules/postgresql_user.py

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

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

const PostgreSQL = "PostgreSQL"
const CockroachDB = "CockroachDB"

func (p *PostgresVersion) GetValidPrivs(privType string) []string {
	if p.ProductName == PostgreSQL {
		return map[string][]string{
			"database": {"CREATE", "CONNECT", "TEMPORARY", "TEMP", "ALL"},
			"table":    {"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "ALL"},
		}[privType]
	} else if p.ProductName == CockroachDB {
		return map[string][]string{
			"database": {"CREATE", "CONNECT", "ALL"},
			"table":    {"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "ALL"},
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
var ADJUST_PRIVILEGES_MAP = map[string]map[string]func(*sql.DB, string, string, []string) error{
	"table": {
		"revoke": revokeTablePrivileges,
		"grant":  grantTablePrivileges,
	},
	"database": {
		"revoke": revokeDatabasePrivileges,
		"grant":  grantDatabasePrivileges,
	},
}
var CHECK_PRIVILEGES_MAP = map[string]func(*sql.DB, string, string, []string) ([]string, []string, []string, error){
	"table":    hasTablePrivileges,
	"database": hasDatabasePrivileges,
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

func UpdateUserPrivs(conn *sql.DB, userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv) (bool, error) {
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
		alter := []string{fmt.Sprintf("ALTER USER %q", userName)}
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
		privMap, err := ParsePrivs(dbPriv.Privs, dbPriv.Scope, serverVersion)
		if err != nil {
			return changed, err
		}
		privsChanged, err := adjustPrivileges(conn, userName, privMap)
		changed = changed || privsChanged
		if err != nil {
			return changed, err
		}
	}
	return changed, nil
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

/*
Return the difference between the privileges that a user already has and

	the privileges that they desire to have.
	:returns: tuple of:
	    * privileges that they have and were requested
	    * privileges they currently hold but were not requested
	    * privileges requested that they do not hold
*/
func hasTablePrivileges(conn *sql.DB, user string, table string, privs []string) ([]string, []string, []string, error) {
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

func grantTablePrivileges(conn *sql.DB, user string, table string, privs []string) error {
	_, err := conn.Exec(fmt.Sprintf("GRANT %s ON TABLE %q TO %q", strings.Join(privs, ", "), table, user))
	return err
}

func revokeTablePrivileges(conn *sql.DB, user string, table string, privs []string) error {
	_, err := conn.Exec(fmt.Sprintf("REVOKE %s ON TABLE %q FROM %q", strings.Join(privs, ", "), table, user))
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
func hasDatabasePrivileges(conn *sql.DB, user string, db string, privs []string) ([]string, []string, []string, error) {
	curPrivs, err := GetDatabasePrivileges(conn, user, db)
	return diffPrivs(curPrivs, privs, err)
}

func GetDatabasePrivileges(conn *sql.DB, user string, db string) ([]string, error) {
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

func grantDatabasePrivileges(conn *sql.DB, user string, db string, privs []string) error {
	privsStr := strings.Join(privs, ", ")
	if user == "PUBLIC" {
		query := fmt.Sprintf("GRANT %s ON DATABASE %q TO PUBLIC", privsStr, db)
		_, err := conn.Exec(query)
		return err
	} else {
		query := fmt.Sprintf("GRANT %s ON DATABASE %q TO %q", privsStr, db, user)
		_, err := conn.Exec(query)
		return err
	}
}

func revokeDatabasePrivileges(conn *sql.DB, user string, db string, privs []string) error {
	privsStr := strings.Join(privs, ", ")
	if user == "PUBLIC" {
		query := fmt.Sprintf("REVOKE %s ON DATABASE %q FROM PUBLIC", privsStr, db)
		_, err := conn.Exec(query)
		return err
	} else {
		query := fmt.Sprintf("REVOKE %s ON DATABASE %q FROM %q", privsStr, db, user)
		_, err := conn.Exec(query)
		return err
	}
}

func adjustPrivileges(conn *sql.DB, user string, privsMapMap map[string]map[string][]string) (bool, error) {
	if len(privsMapMap) == 0 {
		return false, nil
	}
	changed := false
	errors := []error{}
	for privType, privsMap := range privsMapMap {
		for name, privs := range privsMap {
			checkFun := CHECK_PRIVILEGES_MAP[privType]
			_, otherCurrent, desired, err := checkFun(conn, user, name, privs)
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
