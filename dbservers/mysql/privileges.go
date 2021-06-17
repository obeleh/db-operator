package mysql

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	version "github.com/hashicorp/go-version"
	"github.com/kabisa/db-operator/dbservers/query_utils"
	"github.com/thoas/go-funk"
)

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

func isSslKey(key string) bool {
	return key == "CIPHER" || key == "ISSUER" || key == "SUBJECT"
}

func sanitizeRequires(tlsRequires map[string]string) TlsRequires {
	sanitizedRequires := map[string]string{}
	if len(tlsRequires) > 0 {
		for key, value := range tlsRequires {
			sanitizedRequires[strings.ToUpper(key)] = value
		}
		for key, _ := range tlsRequires {
			if isSslKey(key) {
				delete(sanitizedRequires, "SSL")
				delete(sanitizedRequires, "X509")
				return TlsRequires{RequiresMap: sanitizedRequires, RequiresStr: nil}
			}
		}
		_, exists := sanitizedRequires["X509"]
		var reqStr string
		if exists {
			reqStr = "X509"
		} else {
			reqStr = "SSL"
		}
		return TlsRequires{RequiresMap: nil, RequiresStr: &reqStr}
	}
	return TlsRequires{}
}

func (t TlsRequires) Mogrify(query string, params []interface{}) (string, []interface{}) {
	if t.HasTlsRequirements() {
		var requiresQuery string
		if t.RequiresMap != nil {
			keys := funk.Keys(t.RequiresMap).([]string)
			values := funk.Values(t.RequiresMap).([]interface{})
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
	Version    *version.Version
}

func NewServerInfo(versionStr string) (*ServerInfo, error) {
	version, err := version.NewVersion(versionStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse version %s", err)
	}
	serverInfo := ServerInfo{Version: version}
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
	return NewServerInfo(versionStr)
}

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
	requireLine = query_utils.GetStringInBetween(requireLine, "REQUIRES", "PASSWORD")
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

func getGrants(conn *sql.DB, user string, host string) ([]string, error) {
	grants, err := query_utils.SelectFirstValueStringSlice(conn, fmt.Sprintf("SHOW GRANTS FOR %s@%s", user, host))
	if err != nil {
		return nil, fmt.Errorf("Failed getting grants  %s", err)
	}
	filtered := funk.FilterString(grants, func(s string) bool {
		return strings.Contains(s, "ON *.*")
	})
	grantsLine := query_utils.GetStringInBetween(filtered[0], "GRANT", "ON")
	return strings.Split(grantsLine, ", "), nil
}

func CreateUser(conn *sql.DB, user string, host string, password string, encrypted bool, tlsRequires *TlsRequires) error {
	serverInfo, err := getServerInfo(conn)

	if err != nil {
		return fmt.Errorf("Unable to create user %s", err)
	}
	return CreateUserSi(conn, user, host, password, encrypted, serverInfo, tlsRequires)
}

func CreateUserSi(conn *sql.DB, user string, host string, password string, encrypted bool, serverInfo *ServerInfo, tlsRequires *TlsRequires) error {
	oldUserMgmt := serverInfo.UseOldUserMgmt()
	var query string
	var params []interface{}
	var err error

	if len(password) == 0 {
		return fmt.Errorf("password is required")
	}

	if encrypted {
		if serverInfo.SupportsIdentifiedByPassword() {
			query = "CREATE USER %s@%s IDENTIFIED BY PASSWORD %s"
		} else {
			query = "CREATE USER %s@%s IDENTIFIED WITH mysql_native_password AS %s"
		}
	} else {
		if oldUserMgmt {
			query = "CREATE USER %s@%s IDENTIFIED BY %s"
		} else {
			password, err = query_utils.SelectFirstValueString(conn, fmt.Sprintf("SELECT CONCAT('*', UCASE(SHA1(UNHEX(SHA1(%s)))))", password))
			if err != nil {
				return fmt.Errorf("unable to create password for user %s %s", user, err)
			}
			query = "CREATE USER %s@%s IDENTIFIED WITH mysql_native_password AS %s"
		}
	}

	params = []interface{}{user, host, password}
	if oldUserMgmt {
		query, params = tlsRequires.Mogrify(query, params)
	}
	query = fmt.Sprintf(query, params...)
	_, err = conn.Exec(query)
	return err
}

/*
def privileges_unpack(priv, mode):
    """ Take a privileges string, typically passed as a parameter, and unserialize
    it into a dictionary, the same format as privileges_get() above. We have this
    custom format to avoid using YAML/JSON strings inside YAML playbooks. Example
    of a privileges string:
     mydb.*:INSERT,UPDATE/anotherdb.*:SELECT/yetanother.*:ALL
    The privilege USAGE stands for no privileges, so we add that in on *.* if it's
    not specified in the string, as MySQL will always provide this by default.
    """
    if mode == 'ANSI':
        quote = '"'
    else:
        quote = '`'
    output = {}
    privs = []
    for item in priv.strip().split('/'):
        pieces = item.strip().rsplit(':', 1)
        dbpriv = pieces[0].rsplit(".", 1)

        # Check for FUNCTION or PROCEDURE object types
        parts = dbpriv[0].split(" ", 1)
        object_type = ''
        if len(parts) > 1 and (parts[0] == 'FUNCTION' or parts[0] == 'PROCEDURE'):
            object_type = parts[0] + ' '
            dbpriv[0] = parts[1]

        # Do not escape if privilege is for database or table, i.e.
        # neither quote *. nor .*
        for i, side in enumerate(dbpriv):
            if side.strip('`') != '*':
                dbpriv[i] = '%s%s%s' % (quote, side.strip('`'), quote)
        pieces[0] = object_type + '.'.join(dbpriv)

        if '(' in pieces[1]:
            output[pieces[0]] = re.split(r',\s*(?=[^)]*(?:\(|$))', pieces[1].upper())
            for i in output[pieces[0]]:
                privs.append(re.sub(r'\s*\(.*\)', '', i))
        else:
            output[pieces[0]] = pieces[1].upper().split(',')
            privs = output[pieces[0]]

        # Handle cases when there's privs like GRANT SELECT (colA, ...) in privs.
        output[pieces[0]] = normalize_col_grants(output[pieces[0]])

        new_privs = frozenset(privs)
        if not new_privs.issubset(VALID_PRIVS):
            raise InvalidPrivsError('Invalid privileges specified: %s' % new_privs.difference(VALID_PRIVS))

    if '*.*' not in output:
        output['*.*'] = ['USAGE']

    return output
*/

/*
func privilegesUnpack(priv string, mode string) map[string][]string {
	var quote string
	if mode == "ANSI" {
		quote = "\""
	} else {
		quote = "`"
	}
	output := map[string][]string{}
	privs := []string{}

	priv = strings.TrimSpace(priv)

	for item := range strings.Split(priv, "/") {
		allSplit := strings.Split(strings.TrimSpace(item))
		//pieces =   .rsplit(':', 1)
        dbpriv = pieces[0].rsplit(".", 1)
	}
}

func Rsplit(s string, sep string, count int) []string{
	parts := strings.Split(s, sep)
	if len(parts) == ?
}

func adjustPrivileges(conn *sql.DB, user string, privsMapMap map[string]map[string][]string) (bool, error) {
	mode, err := getMode(conn)
	if err != nil {
		return false, fmt.Errorf("failed to get mode for adjustPrivileges %s", err)
	}

}

*/
