// https://github.com/ansible-collections/community.postgresql/blob/main/plugins/modules/postgresql_user.py
package controllers

import (
	"database/sql"
	"fmt"
	"strings"

	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	funk "github.com/thoas/go-funk"
)

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

var VALID_PRIVS = map[string][]string{
	"database": []string{"CREATE", "CONNECT", "TEMPORARY", "TEMP", "ALL"},
	"table":    []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "ALL"},
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

func NormalizePrivileges(privs []string, privType string) []string {
	newPrivs := privs
	if funk.Contains(newPrivs, "ALL") {
		newPrivs = append(newPrivs, VALID_PRIVS[privType]...)
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

func ParsePrivs(privs string, db string) (map[string]map[string][]string, error) {
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

		if !funk.Subset(privSet, VALID_PRIVS[privType]) {
			invalidPrivs := strings.Join(funk.Subtract(privSet, VALID_PRIVS[privType]).([]string), " ")
			return nil, fmt.Errorf("invalid privs specified for %s: %s", privType, invalidPrivs)
		}
		privSet = NormalizePrivileges(privSet, privType)
		oPrivs[privType][name] = privSet
	}
	return oPrivs, nil
}

func UpdateUserPrivs(conn *sql.DB, userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv) error {
	// Server Privs
	maps, err := SelectToArrayMap(conn, "SELECT * FROM pg_roles WHERE rolname=?", userName)
	if err != nil {
		return err
	}
	currentRoleAttrs := maps[0]
	var roleAttrFlagsChanging bool = false
	// TODO Scan server version and add to parser
	roleAttrFlags, err := ParseRoleAttrs(serverPrivs, 0)
	if err != nil {
		return err
	}
	if len(roleAttrFlags) > 0 {
		for _, flag := range roleAttrFlags {
			roleAttrValue := !strings.HasPrefix(flag, "NO")
			if currentRoleAttrs[PRIV_TO_AUTHID_COLUMN[flag]] != roleAttrValue {
				roleAttrFlagsChanging = true
			}
		}
	}

	if roleAttrFlagsChanging {
		alter := []string{fmt.Sprintf("ALTER USER %q", userName)}
		if len(roleAttrFlags) > 0 {
			alter = append(alter, fmt.Sprintf("WITH %s", strings.Join(roleAttrFlags, " ")))
		}

		_, err = conn.Exec(fmt.Sprintf(strings.Join(alter, " ")))
		if err != nil {
			return err
		}
	}

	//DB privs
	for _, dbPriv := range dbPrivs {
		_, err := ParsePrivs(dbPriv.Priv, dbPriv.DbName)
		if err != nil {
			return err
		}

		/*
			databasePrivs := privMap["database"]
			tablePrivs := privMap["tabl"]
		*/
	}
	return nil
}

func getTablePrivileges(conn *sql.DB, user string, table string) ([]string, error) {
	var schema string
	if strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 1)
		schema, table = parts[0], parts[1]
	} else {
		schema = "public"
	}

	query := "SELECT privilege_type FROM information_schema.role_table_grants WHERE grantee=? AND table_name=? AND table_schema=?"
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
func hasTablePrivileges(conn *sql.DB, user string, table string, privs string) ([]string, []string, []string, error) {
	curPrivs, err := getTablePrivileges(conn, user, table)
	if err != nil {
		return nil, nil, nil, err
	}
	haveCurrently := funk.Intersect(curPrivs, privs).([]string)
	otherCurrent, desired := funk.Difference(curPrivs, privs)
	return haveCurrently, otherCurrent.([]string), desired.([]string), nil
}

func grantTablePrivileges(conn *sql.DB, user string, table string, privs string) error {
	// Note: priv escaped by parse_privs
	_, err := conn.Exec("GRANT ? ON TABLE ? TO ?", privs, table, user)
	return err
}
