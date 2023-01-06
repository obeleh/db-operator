package mysql

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	version "github.com/hashicorp/go-version"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/obeleh/db-operator/dbservers/query_utils"
	"github.com/thoas/go-funk"
)

/*
Example of privileges on Host level:
priv: '*.*:ALL' -> all database privileges
priv: '*.*:ALL,GRANT' -> all database privileges with grant option
priv: '*.*:REQUIRESSL' -> Note that REQUIRESSL is a special privilege that should only apply to *.* by itself. Setting this privilege in this manner is deprecated. Use 'tls_requires' instead.

Example of privileges on DB level:
priv:
	'db1.*': 'ALL,GRANT'
	'db2.*': 'ALL,GRANT'

priv:
	FUNCTION my_db.my_function: EXECUTE

tls_requires: -> Modify user to require TLS connection with a valid client certificate
	x509:

tls_requires: -> Modify user to require TLS connection with a specific client certificate and cipher
	subject: '/CN=alice/O=MyDom, Inc./C=US/ST=Oregon/L=Portland'
	cipher: 'ECDHE-ECDSA-AES256-SHA384'

tls_requires: -> Modify user to no longer require SSL

Example privileges string format
mydb.*:INSERT,UPDATE/anotherdb.*:SELECT/yetanotherdb.*:ALL


*/

// https://github.com/ansible-collections/community.mysql/blob/main/plugins/modules/mysql_user.py

var VALID_PRIVS = []string{
	"CREATE", "DROP", "GRANT", "GRANT OPTION",
	"LOCK TABLES", "REFERENCES", "EVENT", "ALTER",
	"DELETE", "INDEX", "INSERT", "SELECT", "UPDATE",
	"CREATE TEMPORARY TABLES", "TRIGGER", "CREATE VIEW",
	"SHOW VIEW", "ALTER ROUTINE", "CREATE ROUTINE",
	"EXECUTE", "FILE", "CREATE TABLESPACE", "CREATE USER",
	"PROCESS", "PROXY", "RELOAD", "REPLICATION CLIENT",
	"REPLICATION SLAVE", "SHOW DATABASES", "SHUTDOWN",
	"SUPER", "ALL", "ALL PRIVILEGES", "USAGE",
	"REQUIRESSL", // Deprecated, to be removed in version 3.0.0 ?
	"CREATE ROLE", "DROP ROLE", "APPLICATION_PASSWORD_ADMIN",
	"AUDIT_ADMIN", "BACKUP_ADMIN", "BINLOG_ADMIN",
	"BINLOG_ENCRYPTION_ADMIN", "CLONE_ADMIN", "CONNECTION_ADMIN",
	"ENCRYPTION_KEY_ADMIN", "FIREWALL_ADMIN", "FIREWALL_USER",
	"GROUP_REPLICATION_ADMIN", "INNODB_REDO_LOG_ARCHIVE",
	"NDB_STORED_USER", "PERSIST_RO_VARIABLES_ADMIN",
	"REPLICATION_APPLIER", "REPLICATION_SLAVE_ADMIN",
	"RESOURCE_GROUP_ADMIN", "RESOURCE_GROUP_USER",
	"ROLE_ADMIN", "SESSION_VARIABLES_ADMIN", "SET_USER_ID",
	"SYSTEM_USER", "SYSTEM_VARIABLES_ADMIN", "SYSTEM_USER",
	"TABLE_ENCRYPTION_ADMIN", "VERSION_TOKEN_ADMIN",
	"XA_RECOVER_ADMIN", "LOAD FROM S3", "SELECT INTO S3",
	"INVOKE LAMBDA",
	"ALTER ROUTINE",
	"BINLOG ADMIN",
	"BINLOG MONITOR",
	"BINLOG REPLAY",
	"CONNECTION ADMIN",
	"READ_ONLY ADMIN",
	"REPLICATION MASTER ADMIN",
	"REPLICATION SLAVE ADMIN",
	"SET USER",
	"SHOW_ROUTINE",
	"SLAVE MONITOR",
	"REPLICA MONITOR",
}

// TODO: Does this cover all database versions of "ALL" priveges???
var ALL_PRIVS = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "RELOAD", "SHUTDOWN", "PROCESS", "FILE",
	"REFERENCES", "INDEX", "ALTER", "SHOW DATABASES", "SUPER", "CREATE TEMPORARY TABLES", "LOCK TABLES",
	"EXECUTE", "REPLICATION SLAVE", "REPLICATION CLIENT", "CREATE VIEW", "SHOW VIEW", "CREATE ROUTINE",
	"ALTER ROUTINE", "CREATE USER", "EVENT", "TRIGGER", "CREATE TABLESPACE", "CREATE ROLE", "DROP ROLE",
	"APPLICATION_PASSWORD_ADMIN", "AUDIT_ABORT_EXEMPT", "AUDIT_ADMIN", "AUTHENTICATION_POLICY_ADMIN",
	"BACKUP_ADMIN", "BINLOG_ADMIN", "BINLOG_ENCRYPTION_ADMIN", "CLONE_ADMIN", "CONNECTION_ADMIN",
	"ENCRYPTION_KEY_ADMIN", "FIREWALL_EXEMPT", "FLUSH_OPTIMIZER_COSTS", "FLUSH_STATUS", "FLUSH_TABLES",
	"FLUSH_USER_RESOURCES", "GROUP_REPLICATION_ADMIN", "GROUP_REPLICATION_STREAM", "INNODB_REDO_LOG_ARCHIVE",
	"INNODB_REDO_LOG_ENABLE", "PASSWORDLESS_USER_ADMIN", "PERSIST_RO_VARIABLES_ADMIN", "REPLICATION_APPLIER",
	"REPLICATION_SLAVE_ADMIN", "RESOURCE_GROUP_ADMIN", "RESOURCE_GROUP_USER", "ROLE_ADMIN",
	"SENSITIVE_VARIABLES_OBSERVER", "SERVICE_CONNECTION_ADMIN", "SESSION_VARIABLES_ADMIN", "SET_USER_ID",
	"SHOW_ROUTINE", "SYSTEM_USER", "SYSTEM_VARIABLES_ADMIN", "TABLE_ENCRYPTION_ADMIN", "XA_RECOVER_ADMIN",
}

var GRANTS_RE = regexp.MustCompile("GRANT (?P<privs>.+) ON (?P<on>.+) TO (['`\"]).*(['`\"])@(['`\"]).*(['`\"])( IDENTIFIED BY PASSWORD (['`\"]).+(['`\"]))? ?(?P<lastgroup>.*)")

// tls_requires description:
// - Set requirement for secure transport as a dictionary of requirements (see the examples).
// - Valid requirements are SSL, X509, SUBJECT, ISSUER, CIPHER.
// - SUBJECT, ISSUER and CIPHER are complementary, and mutually exclusive with SSL and X509.
// - U(https://mariadb.com/kb/en/securing-connections-for-client-and-server/#requiring-tls).
//
// The following struct should either contain a requires string or a map of requires, not both
type TlsRequires struct {
	RequiresMap map[string]string
	RequiresStr *string
}

func (t TlsRequires) HasTlsRequirements() bool {
	return t.RequiresMap != nil && t.RequiresStr != nil
}

func getMode(conn *sql.DB) (string, error) {
	rows, err := conn.Query("SELECT @@GLOBAL.sql_mode;")
	if err != nil {
		return "", fmt.Errorf("unable to read sql mode %s", err)
	}
	rows.Next()
	var modeStr string
	err = rows.Scan(&modeStr)
	if err != nil {
		return "", fmt.Errorf("unable to load modeStr %s", err)
	}
	if strings.Contains(modeStr, "ANSI") {
		return "ANSI", nil
	} else {
		return "NOTANSI", nil
	}
}

func userExists(conn *sql.DB, user string, host string, hostAll bool) (bool, error) {
	var err error
	var count int
	if hostAll {
		count, err = query_utils.SelectFirstValueInt(conn, "SELECT count(*) FROM mysql.user WHERE user = ?", user)
	} else {
		count, err = query_utils.SelectFirstValueInt(conn, "SELECT count(*) FROM mysql.user WHERE user = ? AND host = ?", user, host)
	}
	if err != nil {
		return false, fmt.Errorf("unable to read mysql.user %s", err)
	}
	return count > 0, nil
}

func (t TlsRequires) Mogrify(query string, params []string) (string, []string) {
	if t.HasTlsRequirements() {
		var requiresQuery string
		if t.RequiresMap != nil {
			keys := funk.Keys(t.RequiresMap).([]string)
			values := funk.Values(t.RequiresMap).([]string)
			criteria := funk.Map(keys, func(key string) string {
				return fmt.Sprintf("%s %%s", key)
			}).([]string)
			requiresQuery = strings.Join(criteria, " AND ")
			params = append(params, values...)
		} else {
			requiresQuery = *t.RequiresStr
		}
		query = fmt.Sprintf("%s REQUIRE %s", query, requiresQuery)
	}
	return query, params
}

const (
	MYSQL = iota
	MARIADB
)

type ServerType int

type ServerInfo struct {
	ServerType ServerType
	Mode       string
	Quote      string
	Version    *version.Version
}

func NewServerInfo(versionStr string, mode string) (*ServerInfo, error) {
	version, err := version.NewVersion(versionStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse version %s", err)
	}

	var quote string
	if mode == "ANSI" {
		quote = "\""
	} else {
		quote = "`"
	}

	serverInfo := ServerInfo{Version: version, Mode: mode, Quote: quote}
	if strings.Contains(strings.ToLower(versionStr), "mariadb") {
		serverInfo.ServerType = MARIADB
	} else {
		serverInfo.ServerType = MYSQL
	}

	return &serverInfo, nil
}

func (si ServerInfo) UseOldUserMgmt() bool {
	if si.ServerType == MARIADB {
		threshold, _ := version.NewVersion("10.2")
		return si.Version.LessThan(threshold)
	} else {
		threshold, _ := version.NewVersion("5.7")
		return si.Version.LessThan(threshold)
	}
}

func (si ServerInfo) SupportsIdentifiedByPassword() bool {
	if si.ServerType == MARIADB {
		return true
	} else {
		threshold, _ := version.NewVersion("8")
		return si.Version.LessThan(threshold)
	}
}

func getServerInfo(conn *sql.DB) (*ServerInfo, error) {
	versionStr, err := query_utils.SelectFirstValueString(conn, "SELECT VERSION()")
	if err != nil {
		return nil, fmt.Errorf("failed getting server info %s", err)
	}

	mode, err := getMode(conn)
	if err != nil {
		return nil, fmt.Errorf("failed getting mode for server info %s", err)
	}
	return NewServerInfo(versionStr, mode)
}

// Check if TLS is required for the user to connect???
func getTlsRequires(conn *sql.DB, serverInfo ServerInfo, user string, host string) (TlsRequires, error) {
	var query string
	if serverInfo.UseOldUserMgmt() {
		query = fmt.Sprintf("SHOW GRANTS for '%s'@'%s'", user, host)
	} else {
		// CREATE USER 'u'@'%' IDENTIFIED WITH 'caching_sha2_password' REQUIRE NONE PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK PASSWORD HISTORY DEFAULT PASSWORD REUSE INTERVAL DEFAULT PASSWORD REQUIRE CURRENT DEFAULT
		query = fmt.Sprintf("SHOW CREATE USER '%s'@'%s'", user, host)
	}

	grants, err := query_utils.SelectFirstValueStringSlice(conn, query)
	if err != nil {
		return TlsRequires{}, fmt.Errorf("unable to read user grants for TLS requires %s", err)
	}
	requireList := funk.Filter(grants, func(s string) bool {
		return strings.Contains(s, "REQUIRE")
	}).([]string)
	var requireLine string
	if len(requireList) == 0 {
		return TlsRequires{}, nil
	}
	requireLine = requireList[0]
	requireLine = query_utils.GetStringInBetween(requireLine, "REQUIRE", "PASSWORD")
	words := strings.Fields(requireLine)
	firstRequire := words[0]
	if strings.HasPrefix(firstRequire, "NONE") {
		return TlsRequires{}, nil
	}
	if firstRequire == "SSL" || firstRequire == "X509" {
		return TlsRequires{RequiresStr: &firstRequire}, nil
	}

	requiresMap := map[string]string{}
	for _, word := range words {
		requiresMap[word] = word
	}
	return TlsRequires{RequiresMap: requiresMap}, nil
}

func getServerGrants(conn *sql.DB, user string, host string) ([]string, error) {
	grants, err := query_utils.SelectFirstValueStringSlice(conn, fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'", user, host))
	if err != nil {
		return nil, fmt.Errorf("failed getting grants  %s", err)
	}
	filtered := funk.FilterString(grants, func(s string) bool {
		return strings.Contains(s, "ON *.*")
	})
	grantsLine := strings.TrimSpace(query_utils.GetStringInBetween(filtered[0], "GRANT", "ON"))
	return strings.Split(grantsLine, ", "), nil
}

// Create a user, it will first get the information from the server
// This server info will help use understand the features it supports
func CreateUser(conn *sql.DB, user string, host string, password string, tlsRequires *TlsRequires) error {
	serverInfo, err := getServerInfo(conn)

	if err != nil {
		return fmt.Errorf("unable to create user %s", err)
	}
	return CreateUserSi(conn, user, host, password, serverInfo, tlsRequires)
}

func stringArrayToInterfaceArray(input []string) []interface{} {
	new := make([]interface{}, len(input))
	for i, v := range input {
		new[i] = v
	}
	return new
}

// Create a user with server info.
// This server info will help use understand the features it supports
func CreateUserSi(conn *sql.DB, user string, host string, password string, serverInfo *ServerInfo, tlsRequires *TlsRequires) error {
	oldUserMgmt := serverInfo.UseOldUserMgmt()
	var query string
	var err error

	if len(password) == 0 {
		// No password plugin support implemented
		return fmt.Errorf("password is required")
	}

	if oldUserMgmt {
		query = "CREATE USER %s@%s IDENTIFIED BY %s"
	} else {
		password, err = query_utils.SelectFirstValueString(conn, fmt.Sprintf("SELECT CONCAT('*', UCASE(SHA1(UNHEX(SHA1(%s)))))", password))
		if err != nil {
			return fmt.Errorf("unable to create password for user %s %s", user, err)
		}
		query = "CREATE USER %s@%s IDENTIFIED WITH mysql_native_password AS %s"
	}

	params := []string{user, host, password}
	if oldUserMgmt {
		query, params = tlsRequires.Mogrify(query, params)
	}
	query = fmt.Sprintf(query, stringArrayToInterfaceArray(params)...)
	_, err = conn.Exec(query)
	return err
}

func Rsplit(s string, sep string, count int) []string {
	if count == 0 {
		return []string{s}
	}
	parts := strings.Split(s, sep)
	splitsAvailable := len(parts) - 1
	if splitsAvailable == 0 {
		return []string{s}
	}

	var maxSplits int
	if splitsAvailable < count {
		maxSplits = splitsAvailable
	} else {
		maxSplits = count
	}
	splitLoc := len(parts) - maxSplits
	firstPart := strings.Join(parts[:splitLoc], sep)
	slice := []string{firstPart}
	return append(slice, parts[splitLoc:]...)
}

func parsePrivPiece(piece string) ([]string, []string) {
	inParens := false
	currentItemStripped := ""
	currentItem := ""
	privsStripped := []string{}
	privs := []string{}
	for _, char := range piece {
		if inParens {
			if char == ')' {
				inParens = false
			}
			currentItem += string(char)
		} else {
			if char == ',' {
				privsStripped = append(privsStripped, currentItemStripped)
				currentItemStripped = ""
				privs = append(privs, currentItem)
				currentItem = ""
			} else if char == '(' {
				inParens = true
				currentItem += string(char)
			} else {
				currentItemStripped += string(char)
				currentItem += string(char)
			}
		}
	}
	if len(currentItemStripped) > 0 {
		privsStripped = append(privsStripped, currentItemStripped)
	}
	if len(currentItem) > 0 {
		privs = append(privs, currentItem)
	}
	return privs, privsStripped
}

/*
Check if there is a statement like SELECT (colA, colB)

	in the privilege list.

Return (start index, end index).
*/
func hasGrantOnCol(privileges []string, grant string) (*int, *int) {
	var start *int
	var end *int
	for n, priv := range privileges {
		if strings.Contains(priv, fmt.Sprintf("%s (", grant)) {
			// We found the start element
			curN := n
			start = &curN
		}

		if start != nil && strings.Contains(priv, ")") {
			// We found the end element
			curN := n
			end = &curN
			break
		}
	}

	if start != nil && end != nil {
		// if the privileges list consist of, for example,
		// ['SELECT (A', 'B), 'INSERT'], return indexes of related elements
		return start, end
	}

	// If start and end position is the same element,
	// it means there's expression like 'SELECT (A)',
	// so no need to handle it
	return nil, nil
}

/*
Sort column order in grants like SELECT (colA, colB, ...).

MySQL changes columns order like below:
---------------------------------------
mysql> GRANT SELECT (testColA, testColB), INSERT ON `testDb`.`testTable` TO 'testUser'@'localhost';
Query OK, 0 rows affected (0.04 sec)

mysql> flush privileges;
Query OK, 0 rows affected (0.00 sec)

mysql> SHOW GRANTS FOR testUser@localhost;
+---------------------------------------------------------------------------------------------+
| Grants for testUser@localhost                                                               |
+---------------------------------------------------------------------------------------------+
| GRANT USAGE ON *.* TO 'testUser'@'localhost'                                                |
| GRANT SELECT (testColB, testColA), INSERT ON `testDb`.`testTable` TO 'testUser'@'localhost' |
+---------------------------------------------------------------------------------------------+

We should sort columns in our statement, otherwise the module always will return
that the state has changed.
*/
func sortColumnOrder(statement string) string {
	/*
		1. Extract stuff inside ()
		2. Split
		3. Sort
		4. Put between () and return
	*/

	// "SELECT/UPDATE/.. (colA, colB) => "colA, colB"
	tmp := strings.Split(statement, "(")
	privName := tmp[0]
	columnsStr := strings.TrimRight(tmp[1], ")")

	// "colA, colB" => ["colA", "colB"]
	columns := strings.Split(columnsStr, ",")

	for i, col := range columns {
		col = strings.TrimSpace(col)
		columns[i] = strings.Trim(col, "`")
	}

	sort.Strings(columns)
	return fmt.Sprintf("%s(%s)", privName, strings.Join(columns, ", "))
}

/*
Handle cases when the privs like SELECT (colA, ...) is in the privileges list.
When the privileges list look like ['SELECT (colA,', 'colB)']
(Notice that the statement is splitted)
*/
func handleGrantOnCol(privileges []string, start int, end int) []string {
	var output = []string{}
	if start != end {
		copy(output, privileges[:start])
		selectOnCol := strings.Join(privileges[start:end+1], ", ")
		selectOnCol = sortColumnOrder(selectOnCol)
		output = append(output, selectOnCol)
		output = append(output, privileges[end+1:]...)
	} else {
		// When it looks like it should be, e.g. ['SELECT (colA, colB)'],
		// we need to be sure, the columns is sorted
		copy(output, privileges)
		output[start] = sortColumnOrder(output[start])
	}

	return output
}

/*
Fix and sort grants on columns in privileges list

Make ['SELECT (A, B)', 'INSERT (A, B)', 'DETELE']
from ['SELECT (A', 'B)', 'INSERT (B', 'A)', 'DELETE'].
*/
func normalizeColGrants(privileges []string) []string {
	for _, grant := range []string{"SELECT", "UPDATE", "INSERT", "REFERENCES"} {
		start, end := hasGrantOnCol(privileges, grant)
		if start != nil {
			privileges = handleGrantOnCol(privileges, *start, *end)
		}
	}
	return privileges
}

/*
Take a privileges string, typically passed as a parameter, and unserialize
it into a dictionary, the same format as privileges_get() above. We have this
custom format to avoid using YAML/JSON strings inside YAML playbooks. Example
of a privileges string:

	mydb.*:INSERT,UPDATE/anotherdb.*:SELECT/yetanother.*:ALL

The privilege USAGE stands for no privileges, so we add that in on *.* if it's
not specified in the string, as MySQL will always provide this by default.
*/
func privilegesUnpack(dbPrivs []dboperatorv1alpha1.DbPriv, mode string) (map[string][]string, error) {
	var quote string
	if mode == "ANSI" {
		quote = "\""
	} else {
		quote = "`"
	}
	output := map[string][]string{}

	for _, item := range dbPrivs {
		dbPriv := strings.Split(item.DbName, ".")

		// Check for FUNCTION or PROCEDURE object types
		parts := strings.SplitN(item.Privs, " ", 2)
		objectType := ""
		if len(parts) > 1 && (parts[0] == "FUNCTION" || parts[0] == "PROCEDURE") {
			objectType = parts[0] + " "
			dbPriv[0] = parts[1]
		}

		// Do not escape if privilege is for database or table, i.e.
		// neither quote *. nor .*
		for i, side := range dbPriv {
			if strings.Trim(side, "`") != "*" {
				dbPriv[i] = fmt.Sprintf("%s%s%s", quote, strings.TrimSpace(side), quote)
			}
		}
		item.DbName = objectType + strings.Join(dbPriv, ".")
		privs, privsStripped := parsePrivPiece(strings.ToUpper(item.Privs))
		output[item.DbName] = privs

		invalidPrivs := funk.Subtract(privsStripped, VALID_PRIVS).([]string)
		if len(invalidPrivs) > 0 {
			return nil, fmt.Errorf("invalid privileges found %s", invalidPrivs)
		}

		// Handle cases when there's privs like GRANT SELECT (colA, ...) in privs.
		output[item.DbName] = normalizeColGrants(output[item.DbName])
	}

	_, exists := output["*.*"]
	if !exists {
		output["*.*"] = []string{"USAGE"}
	}

	return output, nil
}

func privilegesRevoke(conn *sql.DB, user string, host string, dbTable string, priv []string, grantOption bool) error {
	if isQuoted(host) || isQuoted(user) {
		return fmt.Errorf("quoted user or host")
	}
	if dbTable != "*.*" && !isQuoted(dbTable) {
		return fmt.Errorf("unquoted dbTable")
	}

	// Escape '%' since mysql db.execute() uses a format string
	dbTable = strings.ReplaceAll(dbTable, "%", "%%")
	if grantOption {
		// Note this doesn't escape well, I _suspect_ parametrized queries won't work here
		revokeGrantQuery := fmt.Sprintf("REVOKE GRANT OPTION ON ? FROM '%s'@'%s';", user, host)
		_, err := conn.Exec(revokeGrantQuery, dbTable)
		if err != nil {
			return err
		}
	}

	nonGrantPrivs := funk.FilterString(priv, func(s string) bool {
		return s != "GRANT"
	})
	privStr := strings.Join(nonGrantPrivs, ",")
	revokePrivQuery := fmt.Sprintf("REVOKE %s ON %s FROM '%s'@'%s';", privStr, dbTable, user, host)
	_, err := conn.Exec(revokePrivQuery)
	return err
}

func isQuoted(input string) bool {
	return input[0] == '"' || input[0] == '`'
}

func privilegesGrant(conn *sql.DB, user string, host string, dbTable string, priv []string, tlsRequires TlsRequires, si ServerInfo) error {
	if isQuoted(host) || isQuoted(user) {
		return fmt.Errorf("quoted user or host")
	}
	if dbTable != "*.*" && !isQuoted(dbTable) {
		return fmt.Errorf("unquoted dbTable")
	}

	// Escape '%' since mysql db.execute uses a format string and the
	// specification of db and table often use a % (SQL wildcard)
	dbTable = strings.ReplaceAll(dbTable, "%", "%%")
	nonGrantPrivs := funk.FilterString(priv, func(s string) bool {
		return s != "GRANT"
	})
	privStr := strings.Join(nonGrantPrivs, ",")
	grantPrivQuery := fmt.Sprintf("GRANT %s ON %s TO '%s'@'%s';", privStr, dbTable, user, host)
	if tlsRequires.HasTlsRequirements() && si.UseOldUserMgmt() {
		params := []string{user, host}
		tlsRequires.Mogrify(grantPrivQuery, params)
	}
	if funk.Contains(priv, "GRANT") {
		grantPrivQuery += " WITH GRANT OPTION"
	}

	_, err := conn.Exec(grantPrivQuery)
	return err
}

func byteSliceSliceToStringSlice(inp [][]byte) []string {
	output := make([]string, len(inp))
	for i, val := range inp {
		output[i] = string(val)
	}
	return output
}

func getPrivileges(conn *sql.DB, userName string, host string) (map[string][]string, error) {
	query := fmt.Sprintf("SHOW GRANTS for '%s'@'%s';", userName, host)
	rows, err := conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("unable to read databases from server %s", err)
	}

	// first build a map to gather all privs per db
	grants := map[string][]string{}

	for rows.Next() {
		var grant string
		err = rows.Scan(&grant)
		if err != nil {
			return grants, err
		}

		byteGroups := GRANTS_RE.FindSubmatch([]byte(grant))
		stringGroups := byteSliceSliceToStringSlice(byteGroups)
		if len(stringGroups) != 11 {
			return grants, fmt.Errorf("getPrivileges: Regex string did not match expected format")
		}
		privileges := strings.Split(stringGroups[1], ",")

		for i, priv := range privileges {
			privileges[i] = strings.TrimSpace(priv)
			if privileges[i] == "ALL PRIVILEGES" {
				privileges[i] = "ALL"
			}
		}

		/* Handle cases when there's privs like GRANT SELECT (colA, ...) in privs.
		To this point, the privileges list can look like
		['SELECT (`A`', '`B`)', 'INSERT'] that is incorrect (SELECT statement is splitted).
		Columns should also be sorted to compare it with desired privileges later.
		Determine if there's a case similar to the above:
		*/
		privileges = normalizeColGrants(privileges)

		if strings.Contains(stringGroups[3], "WITH GRANT OPTION") {
			privileges = append(privileges, "GRANT")
		}
		db := stringGroups[2]

		prevPrivs, exists := grants[db]
		if exists {
			grants[db] = append(prevPrivs, privileges...)
		} else {
			grants[db] = privileges
		}
	}

	return grants, nil
}

func UpdateUserPrivs(conn *sql.DB, userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv) (bool, error) {
	host := "%" // Not yet supporting other host names in crd spec
	si, err := getServerInfo(conn)
	if err != nil {
		return false, fmt.Errorf("failed to getServerInfo for UpdateUserPrivs %s", err)
	}

	tlsRequires, err := getTlsRequires(conn, *si, userName, host)
	if err != nil {
		return false, fmt.Errorf("failed to getTlsRequires for UpdateUserPrivs %s", err)
	}

	curPrivs, err := getPrivileges(conn, userName, host)
	if err != nil {
		return false, fmt.Errorf("failed to UpdateUserPrivs %s", err)
	}

	var allPrivs []dboperatorv1alpha1.DbPriv
	if len(serverPrivs) == 0 {
		allPrivs = dbPrivs
	} else {
		allPrivs = append(dbPrivs, dboperatorv1alpha1.DbPriv{DbName: "*.*", Privs: serverPrivs})
	}

	desiredPrivs, err := privilegesUnpack(allPrivs, si.Mode)
	if err != nil {
		return false, fmt.Errorf("failed to privilegesUnpack desiredPrivs %s", err)
	}

	changes := false
	for dbTable, curDbTablePrivs := range curPrivs {
		desiredDbTablePrivs, desiredFound := desiredPrivs[dbTable]
		if !desiredFound {
			desiredDbTablePrivs = []string{}
		}

		if len(curDbTablePrivs) > 50 {
			subtracted := funk.Subtract(ALL_PRIVS, curDbTablePrivs).([]string)
			if len(subtracted) == 0 {
				curDbTablePrivs = []string{"ALL"}
				curPrivs[dbTable] = curDbTablePrivs
			} else {
				panic("Not entirely sure if current list of privileges is equal to ALL privileges")
			}
		}

		grantOption := false
		grantIndex := funk.IndexOfString(curDbTablePrivs, "GRANT")
		if grantIndex != -1 {
			grantOption = true
			curDbTablePrivs = funk.DropString(curDbTablePrivs, grantIndex)
		}

		toRevoke := funk.SubtractString(curDbTablePrivs, desiredDbTablePrivs)
		if len(toRevoke) > 0 {
			privilegesRevoke(conn, userName, host, dbTable, toRevoke, grantOption)
			if err != nil {
				return changes, err
			}
			changes = true
		}
	}

	for dbTable, desiredDbTablePrivs := range desiredPrivs {
		curDbTablePrivs, exists := curPrivs[dbTable]

		if !exists {
			curDbTablePrivs = []string{}
		}

		toGrant := funk.SubtractString(desiredDbTablePrivs, curDbTablePrivs)
		if len(toGrant) > 0 {
			err = privilegesGrant(conn, userName, host, dbTable, toGrant, tlsRequires, *si)
			if err != nil {
				return changes, err
			}
			changes = true
		}
	}
	return changes, nil
}
