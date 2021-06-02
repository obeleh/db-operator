package postgres_test

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	"github.com/kabisa/db-operator/dbservers/postgres"
	"github.com/thoas/go-funk"
)

func TestParseRoleAttrs(t *testing.T) {
	bla, err := postgres.ParseRoleAttrs("superuser,LOGIN,", 0)
	if err != nil {
		t.Errorf("Got error parsing Role Attrs %s", err)
	}
	expected := []string{"SUPERUSER", "LOGIN"}
	if !funk.Equal(bla, expected) {
		t.Errorf("ParseRoleAttrs should")
	}
}

func TestParseRoleAttrsInvalidAttrs(t *testing.T) {
	_, err := postgres.ParseRoleAttrs("INVALIDPARAM,LOGIN,", 0)
	if err == nil {
		t.Error("expected error from parsing invalid param")
	}
	errorString := fmt.Sprintf("%s", err)
	if errorString != "invalid role_attr_flags specified: INVALIDPARAM" {
		t.Errorf("got %s, expected bla", errorString)
	}
}

func TestNormalizeDatabasePrivileges(t *testing.T) {
	privs := postgres.NormalizePrivileges([]string{"ALL", "CONNECT"}, "database")

	expected := []string{"CREATE", "CONNECT", "TEMPORARY"}
	missing, unExpected := funk.Difference(expected, privs)
	if len(missing.([]string)) > 0 {
		t.Errorf("expected %s privs", missing)
	}
	if len(unExpected.([]string)) > 0 {
		t.Errorf("got unexpected %s privs", unExpected)
	}
}

func TestNormalizeTablePrivileges(t *testing.T) {
	privs := postgres.NormalizePrivileges([]string{"ALL", "INSERT"}, "table")

	expected := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"}
	missing, unExpected := funk.Difference(expected, privs)
	if len(missing.([]string)) > 0 {
		t.Errorf("expected %s privs", missing)
	}
	if len(unExpected.([]string)) > 0 {
		t.Errorf("got unexpected %s privs", unExpected)
	}
}

func TestParsePrivs(t *testing.T) {
	privMap, err := postgres.ParsePrivs("ALL/test_table:select,delete", "testdb")
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}

	expected := map[string]map[string][]string{
		"database": {
			"testdb": []string{"CREATE", "CONNECT", "TEMPORARY"},
		},
		"table": {
			"test_table": []string{"SELECT", "DELETE"},
		},
	}
	if !funk.Equal(privMap, expected) {
		t.Errorf("got %s expected %s", privMap, expected)
	}
}

func TestUpdateUserPrivs(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expected := sqlmock.NewRows([]string{
		"rolname", "rolsuper", "rolinherit", "rolcreaterole", "rolcreatedb", "rolcanlogin", "rolreplication", "rolconnlimit", "rolpassword", "rolvaliduntil", "olbypassrls", "rolconfig", "oid",
	})
	expected.AddRow("testuser", false, true, false, false, false, false, nil, -1, "********", nil, false, 1638)
	mock.ExpectQuery(
		"SELECT * FROM pg_roles WHERE rolname=$1",
	).WithArgs(
		"testuser",
	).WillReturnRows(expected)
	mock.ExpectExec(
		fmt.Sprintf("ALTER USER %q WITH CREATEDB", "testuser"),
	).WillReturnResult(sqlmock.NewResult(1, 1))

	dbPrivs := []dboperatorv1alpha1.DbPriv{
		{
			DbName: "testdb",
			Privs:  "ALL",
		},
	}
	_, err = postgres.UpdateUserPrivs(db, "testuser", "CREATEDB", dbPrivs)

	if err != nil {
		t.Errorf("unexpected error updating userprivs %s", err)
	}
}

func TestGetDatabasePrivilegest(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expected := sqlmock.NewRows([]string{"datacl"})
	expected.AddRow("{=Tc/postgres,postgres=CTc/postgres,testuser=CTc/postgres}")
	mock.ExpectQuery("SELECT datacl FROM pg_database WHERE datname = $1").WithArgs(
		"testdb",
	).WillReturnRows(expected)

	privs, err := postgres.GetDatabasePrivileges(db, "testuser", "testdb")
	if err != nil {
		t.Errorf("database privileges failed %s", err)
	}
	expectedPrivs := []string{"CREATE", "TEMPORARY", "CONNECT"}
	if !funk.Equal(privs, expectedPrivs) {
		t.Errorf("got unexpected privileges %s expected %s", privs, expectedPrivs)
	}
}
