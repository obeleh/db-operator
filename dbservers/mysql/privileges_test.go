package mysql

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/thoas/go-funk"
)

func expectSQLQuery(mock sqlmock.Sqlmock, qry, column, row string) {
	mockOutput := sqlmock.NewRows([]string{column})
	mockOutput.AddRow(row)
	mock.ExpectQuery(qry).WillReturnRows(mockOutput)
}

func TestGetTlsRequiresNone(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expectSQLQuery(
		mock,
		"SHOW CREATE USER 'user'@'%'",
		"CREATE USER for user@%",
		"CREATE USER 'user'@'%' IDENTIFIED WITH 'caching_sha2_password' REQUIRE NONE PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK PASSWORD HISTORY DEFAULT PASSWORD REUSE INTERVAL DEFAULT PASSWORD REQUIRE CURRENT DEFAULT",
	)

	serverInfo, _ := NewServerInfo("8.0.25", "ANSI")
	tlsRequires, err := getTlsRequires(db, *serverInfo, "user", "%")

	if err != nil {
		t.Fatalf("failed parsing TLS requirements, %s", err)
	}
	expectedTlsRequires := TlsRequires{}
	if !reflect.DeepEqual(tlsRequires, expectedTlsRequires) {
		t.Fatalf("unexpected requires")
	}
}

func TestGetTlsRequiresTLS(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expectSQLQuery(
		mock,
		"SHOW CREATE USER 'user'@'%'",
		"CREATE USER for user@%",
		"CREATE USER 'user'@'%' IDENTIFIED WITH 'caching_sha2_password' REQUIRE SSL PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK PASSWORD HISTORY DEFAULT PASSWORD REUSE INTERVAL DEFAULT PASSWORD REQUIRE CURRENT DEFAULT",
	)

	serverInfo, _ := NewServerInfo("8.0.25", "ANSI")
	tlsRequires, err := getTlsRequires(db, *serverInfo, "user", "%")

	if err != nil {
		t.Fatalf("failed parsing TLS requirements, %s", err)
	}
	expectedStr := "SSL"
	expectedTlsRequires := TlsRequires{RequiresStr: &expectedStr}
	if !reflect.DeepEqual(tlsRequires, expectedTlsRequires) {
		t.Fatalf("unexpected requires")
	}
}

func TestRsplit5(t *testing.T) {
	res := Rsplit("a,b,c,d", ",", 5)
	expected := []string{
		"a",
		"b",
		"c",
		"d",
	}
	if !funk.Equal(res, expected) {
		t.Fatalf("Split with more count than splits failed")
	}
}

func TestRsplit1(t *testing.T) {
	res := Rsplit("a,b,c,d", ",", 1)
	expected := []string{
		"a,b,c",
		"d",
	}
	if !funk.Equal(res, expected) {
		t.Fatalf("Split with fewer count than splits failed")
	}
}

func TestPrivilegesUnpack(t *testing.T) {
	privs := []dboperatorv1alpha1.DbPriv{
		{
			Scope: "mydb.*",
			Privs: "INSERT,UPDATE",
		},
		{
			Scope: "anotherdb.*",
			Privs: "SELECT(col1,col2),UPDATE",
		},
		{
			Scope: "yetanother.*",
			Privs: "ALL",
		},
	}
	privMap, err := privilegesUnpack(privs, "ANSI")
	if err != nil {
		t.Fatalf("Failed unpacking privileges %s", err)
	}

	expected := map[string][]string{
		"\"mydb\".*":       {"INSERT", "UPDATE"},
		"\"anotherdb\".*":  {"SELECT(COL1,COL2)", "UPDATE"},
		"\"yetanother\".*": {"ALL"},
		"*.*":              {"USAGE"},
	}

	if !reflect.DeepEqual(privMap, expected) {
		t.Fatal("privileges unpacking returned unexpected map of privs")
	}
}

func TestPrivilegesUnpackNonAnsi(t *testing.T) {
	privs := []dboperatorv1alpha1.DbPriv{
		{
			Scope: "mydb.*",
			Privs: "INSERT,UPDATE",
		},
		{
			Scope: "anotherdb.*",
			Privs: "SELECT(col1,col2),UPDATE",
		},
		{
			Scope: "yetanother.*",
			Privs: "ALL",
		},
	}
	privMap, err := privilegesUnpack(privs, "NOTANSI")
	if err != nil {
		t.Fatalf("Failed unpacking privileges %s", err)
	}

	expected := map[string][]string{
		"`mydb`.*":       {"INSERT", "UPDATE"},
		"`anotherdb`.*":  {"SELECT(COL1,COL2)", "UPDATE"},
		"`yetanother`.*": {"ALL"},
		"*.*":            {"USAGE"},
	}

	if !reflect.DeepEqual(privMap, expected) {
		t.Fatal("privileges unpacking returned unexpected map of privs")
	}
}

func TestParsePrivPiece(t *testing.T) {
	result, resultStripped := parsePrivPiece("INSERT,SELECT(col1,col2),UPDATE")
	expected := []string{"INSERT", "SELECT(col1,col2)", "UPDATE"}
	expectedStripped := []string{"INSERT", "SELECT", "UPDATE"}
	if !funk.Equal(result, expected) {
		t.Fatalf("Parsing privlist failed for non stripped parts")
	}
	if !funk.Equal(resultStripped, expectedStripped) {
		t.Fatalf("Parsing privlist failed for stripped parts")
	}
}

func TestGetModeAnsi(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expectSQLModeQuery(mock)

	mode, err := getMode(db)
	if mode != "ANSI" {
		t.Fatal("failed getting mode")
	}

	if err != nil {
		t.Errorf("GetMode failed: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func expectSQLModeQuery(mock sqlmock.Sqlmock) {
	expectSQLQuery(mock, "SELECT @@GLOBAL.sql_mode;", "@@GLOBAL.sql_mode", "ANSI")
}

func expectVersionQuery(mock sqlmock.Sqlmock) {
	expectSQLQuery(mock, "SELECT VERSION();", "VERSION()", "8.0.32")
}

func TestGetModeNotAnsi(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expectSQLQuery(
		mock,
		"SELECT @@GLOBAL.sql_mode;",
		"@@GLOBAL.sql_mode",
		"NO_ENGINE_SUBSTITUTION, NO_AUTO_CREATE_USER",
	)

	mode, err := getMode(db)
	if mode != "NOTANSI" {
		t.Fatal("failed getting mode")
	}

	if err != nil {
		t.Errorf("GetMode failed: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestUserExists(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mockOutput := sqlmock.NewRows([]string{"count(*)"})
	mockOutput.AddRow(1)
	mock.ExpectQuery("SELECT count(*) FROM mysql.user WHERE user = ?").WithArgs("jantje").WillReturnRows(mockOutput)

	mockOutput2 := sqlmock.NewRows([]string{"count(*)"})
	mockOutput2.AddRow(1)
	mock.ExpectQuery("SELECT count(*) FROM mysql.user WHERE user = ? AND host = ?").WithArgs("jantje", "%").WillReturnRows(mockOutput2)

	exists, err := userExists(db, "jantje", "dummy", true)
	if err != nil {
		t.Errorf("userExists failed: %s", err)
	}
	if !exists {
		t.Fatal("Expected user to exist")
	}

	exists, err = userExists(db, "jantje", "%", false)
	if err != nil {
		t.Errorf("userExists failed: %s", err)
	}
	if !exists {
		t.Fatal("Expected user to exist")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestUserDoesntExistForHost(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mockOutput := sqlmock.NewRows([]string{"count(*)"})
	mockOutput.AddRow(0)
	mock.ExpectQuery("SELECT count(*) FROM mysql.user WHERE user = ? AND host = ?").WithArgs("jantje", "myhost").WillReturnRows(mockOutput)

	exists, err := userExists(db, "jantje", "myhost", false)
	if exists {
		t.Fatal("Expected user not to exist")
	}

	if err != nil {
		t.Errorf("userExists failed: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("TestUserDoesntExistForHost: there were unfulfilled expectations: %s", err)
	}
}

func TestPrivilegesGrant(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	tlsReq := TlsRequires{}
	si, err := NewServerInfo("10.3", "ANSI")
	if err != nil {
		t.Errorf("NewServerInfo failed: %s", err)
	}

	mock.ExpectExec("GRANT SELECT ON `chair` TO 'jantje'@'myhost';").WillReturnResult(sqlmock.NewResult(1, 1))

	err = privilegesGrant(db, "jantje", "myhost", "`chair`", []string{"SELECT"}, tlsReq, *si)

	if err != nil {
		t.Errorf("privilegesGrant failed: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("TestPrivilegesGrant: there were unfulfilled expectations: %s", err)
	}
}

func TestPrivilegesRevoke(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mock.ExpectExec("REVOKE GRANT OPTION ON ? FROM 'jantje'@'myhost';").WithArgs("`chair`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("REVOKE SELECT ON `chair` FROM 'jantje'@'myhost';").WillReturnResult(sqlmock.NewResult(1, 1))

	err = privilegesRevoke(db, "jantje", "myhost", "`chair`", []string{"SELECT"}, true)

	if err != nil {
		t.Errorf("privilegesRevoke failed: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("TestPrivilegesRevoke: there were unfulfilled expectations: %s", err)
	}
}

/*
func TestGetPrivileges(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mockOutput := sqlmock.NewRows([]string{"@@GLOBAL.sql_mode"})
	mock.ExpectQuery("SHOW GRANTS for 'USER'@'HOST';").WillReturnRows(mockOutput)
	_, err = getPrivileges(db, "USER", "HOST")
	// if len(privs) != 3 {
	// 	t.Errorf("Expected XXX")
	// }

	if err != nil {
		t.Errorf("getPrivileges failed: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("TestGetPrivileges: there were unfulfilled expectations: %s", err)
	}
}
*/

func TestPropertiesMapToMySQLVersion(t *testing.T) {
	propertiesMap := map[string]interface{}{
		"version":                 "8.0.31",
		"version_comment":         "MySQL Community Server - GPL",
		"version_compile_machine": "aarch64",
		"version_compile_os":      "Linux",
		"version_compile_zlib":    "1.2.12",
	}
	found, err := propertiesMapToMySqlVersion(propertiesMap)
	if err != nil {
		t.Errorf("propertiesMapToMySqlVersion failed: %s", err)
	}

	expected := &MySQLVersion{
		Product:    "MySQL Community Server - GPL",
		VersionStr: "8.0.31",
		Major:      8,
		Minor:      0,
		Patch:      31,
	}

	if !reflect.DeepEqual(found, expected) {
		t.Error("Version parsing for MySQL failed")
	}
}

func TestPropertiesMapToMySQLVersionMariaDb(t *testing.T) {
	propertiesMap := map[string]interface{}{
		"version":                 "5.5.60-MariaDB",
		"version_comment":         "Source distribution",
		"version_compile_machine": "i686",
		"version_compile_os":      "suse-linux",
	}
	found, err := propertiesMapToMySqlVersion(propertiesMap)
	if err != nil {
		t.Errorf("propertiesMapToMySqlVersion failed: %s", err)
	}

	expected := &MySQLVersion{
		Product:    "MariaDB Source distribution",
		VersionStr: "5.5.60-MariaDB",
		Major:      5,
		Minor:      5,
		Patch:      60,
	}

	if !reflect.DeepEqual(found, expected) {
		t.Error("Version parsing for MySQL failed")
	}
}

func TestCreateUser(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	tlsRequires := &TlsRequires{}

	expectVersionQuery(mock)
	expectSQLModeQuery(mock)

	shaPassword := "*920A368754836100C4FC7C6559FEE1D6B9C0005F"

	mockOutput := sqlmock.NewRows([]string{"DUMMY COL"})
	mockOutput.AddRow(shaPassword)
	mock.ExpectQuery("SELECT CONCAT('*', UCASE(SHA1(UNHEX(SHA1(?)))))").WithArgs("paswoord").WillReturnRows(mockOutput)

	mock.ExpectExec("CREATE USER ?@? IDENTIFIED WITH mysql_native_password AS ?;").WithArgs("jantje", "tafel", shaPassword).WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreateUser(db, "jantje", "tafel", "paswoord", tlsRequires)

	if err != nil {
		t.Errorf("TestCreateUser failed: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("TestCreateUser: there were unfulfilled expectations: %s", err)
	}
}
