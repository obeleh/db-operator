package mysql

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	"github.com/thoas/go-funk"
)

func TestGetTlsRequiresNone(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expected := sqlmock.NewRows([]string{
		"CREATE USER for user@%",
	})
	expected.AddRow(
		"CREATE USER 'user'@'%' IDENTIFIED WITH 'caching_sha2_password' REQUIRE NONE PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK PASSWORD HISTORY DEFAULT PASSWORD REUSE INTERVAL DEFAULT PASSWORD REQUIRE CURRENT DEFAULT",
	)
	mock.ExpectQuery(
		"SHOW CREATE USER 'user'@'%'",
	).WillReturnRows(expected)

	serverInfo, _ := NewServerInfo("8.0.25")
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

	expected := sqlmock.NewRows([]string{
		"CREATE USER for user@%",
	})
	expected.AddRow(
		"CREATE USER 'user'@'%' IDENTIFIED WITH 'caching_sha2_password' REQUIRE SSL PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK PASSWORD HISTORY DEFAULT PASSWORD REUSE INTERVAL DEFAULT PASSWORD REQUIRE CURRENT DEFAULT",
	)
	mock.ExpectQuery(
		"SHOW CREATE USER 'user'@'%'",
	).WillReturnRows(expected)

	serverInfo, _ := NewServerInfo("8.0.25")
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
			DbName: "mydb.*",
			Privs:  "INSERT,UPDATE",
		},
		{
			DbName: "anotherdb.*",
			Privs:  "SELECT(col1,col2),UPDATE",
		},
		{
			DbName: "yetanother.*",
			Privs:  "ALL",
		},
	}
	privMap, err := privilegesUnpack(privs, "ANSI")
	if err != nil {
		t.Fatalf("Failed unpacking privileges %s", err)
	}
	print(privMap)
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

	mockOutput := sqlmock.NewRows([]string{"@@GLOBAL.sql_mode"})
	mockOutput.AddRow("ANSI")
	mock.ExpectQuery("SELECT @@GLOBAL.sql_mode;").WillReturnRows(mockOutput)

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

func TestGetModeNotAnsi(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mockOutput := sqlmock.NewRows([]string{"@@GLOBAL.sql_mode"})
	mockOutput.AddRow("NO_ENGINE_SUBSTITUTION, NO_AUTO_CREATE_USER")
	mock.ExpectQuery("SELECT @@GLOBAL.sql_mode;").WillReturnRows(mockOutput)

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

	exists, err := userExists(db, "jantje", "myhost", true)
	if !exists {
		t.Fatal("Expected user to exist")
	}

	if err != nil {
		t.Errorf("userExists failed: %s", err)
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
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
