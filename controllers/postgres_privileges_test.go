package controllers_test

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dboperatorv1alpha1 "github.com/kabisa/db-operator/api/v1alpha1"
	controllers "github.com/kabisa/db-operator/controllers"
	"github.com/thoas/go-funk"
)

func TestParseRoleAttrs(t *testing.T) {
	bla, err := controllers.ParseRoleAttrs("superuser,LOGIN,", 0)
	if err != nil {
		t.Errorf("Got error parsing Role Attrs %s", err)
	}
	expected := []string{"SUPERUSER", "LOGIN"}
	if !funk.Equal(bla, expected) {
		t.Errorf("ParseRoleAttrs should")
	}
}

func TestParseRoleAttrsInvalidAttrs(t *testing.T) {
	_, err := controllers.ParseRoleAttrs("INVALIDPARAM,LOGIN,", 0)
	if err == nil {
		t.Error("expected error from parsing invalid param")
	}
	errorString := fmt.Sprintf("%s", err)
	if errorString != "invalid role_attr_flags specified: INVALIDPARAM" {
		t.Errorf("got %s, expected bla", errorString)
	}
}

func TestNormalizeDatabasePrivileges(t *testing.T) {
	privs := controllers.NormalizePrivileges([]string{"ALL", "CONNECT"}, "database")

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
	privs := controllers.NormalizePrivileges([]string{"ALL", "INSERT"}, "table")

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
	privMap, err := controllers.ParsePrivs("ALL/test_table:select,delete", "testdb")
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
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expected := sqlmock.NewRows([]string{
		"rolname", "rolsuper", "rolinherit", "rolcreaterole", "rolcreatedb", "rolcanlogin", "rolreplication", "rolconnlimit", "rolpassword", "rolvaliduntil", "olbypassrls", "rolconfig", "oid",
	})
	expected.AddRow("testuser", false, true, false, false, false, false, nil, -1, "********", nil, false, 1638)
	mock.ExpectQuery(
		"SELECT * FROM pg_roles WHERE rolname=?",
	).WithArgs(
		"testuser",
	).WillReturnRows(expected)

	dbPrivs := []dboperatorv1alpha1.DbPriv{
		{
			DbName: "testdb",
			Priv:   "ALL",
		},
	}
	err = controllers.UpdateUserPrivs(db, "testuser", "CONNECT", dbPrivs)

	if err != nil {
		t.Errorf("unexpected error updating userprivs %s", err)
	}
}
