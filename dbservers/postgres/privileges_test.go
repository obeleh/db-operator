package postgres

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"
	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	"github.com/thoas/go-funk"
)

const FullPostgresVersionString = "PostgreSQL 15.1 (Debian 15.1-1.pgdg110+1) on aarch64-unknown-linux-gnu, compiled by gcc (Debian 10.2.1-6) 10.2.1 20210110, 64-bit"
const FullCockroachDBVersionString = "CockroachDB CCL v22.2.2 (x86_64-pc-linux-gnu, built 2023/01/04 17:23:00, go1.19.1)"

func TestParseRoleAttrs(t *testing.T) {
	bla, err := ParseRoleAttrs("superuser,LOGIN,", 0)
	if err != nil {
		t.Errorf("Got error parsing Role Attrs %s", err)
	}
	expected := []string{"SUPERUSER", "LOGIN"}
	if !funk.Equal(bla, expected) {
		t.Errorf("ParseRoleAttrs should")
	}
}

func TestParseRoleAttrsInvalidAttrs(t *testing.T) {
	_, err := ParseRoleAttrs("INVALIDPARAM,LOGIN,", 0)
	if err == nil {
		t.Error("expected error from parsing invalid param")
	}
	errorString := fmt.Sprintf("%s", err)
	if errorString != "invalid role_attr_flags specified: INVALIDPARAM" {
		t.Errorf("got %s, expected bla", errorString)
	}
}

func TestNormalizeDatabasePrivileges(t *testing.T) {
	privs := NormalizePrivileges([]string{"ALL", "CONNECT"}, "database", &PostgresVersion{
		ProductName: PostgreSQL,
	})

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
	privs := NormalizePrivileges([]string{"ALL", "INSERT"}, "table", &PostgresVersion{
		ProductName: PostgreSQL,
	})

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
	privMap, err := ParsePrivs("ALL/test_table:select,delete", "testdb", &PostgresVersion{ProductName: PostgreSQL})
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
		"schema": {},
	}
	if !funk.Equal(privMap, expected) {
		t.Errorf("got %s expected %s", privMap, expected)
	}
}

func TestParseSchemaPrivs(t *testing.T) {
	privMap, err := ParsePrivs("USAGE", "testdb.public", &PostgresVersion{ProductName: PostgreSQL})
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}

	expected := map[string]map[string][]string{
		"database": {},
		"table":    {},
		"schema": {
			"testdb.public": []string{"USAGE"},
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
		"SELECT * FROM pg_roles WHERE rolname = $1",
	).WithArgs(
		"testuser",
	).WillReturnRows(expected)
	addVersionQueryToMock(mock, FullPostgresVersionString)
	mock.ExpectExec(
		fmt.Sprintf("ALTER USER %q WITH CREATEDB", "testuser"),
	).WillReturnResult(sqlmock.NewResult(1, 1))

	expectGetDatabasePrivileges(mock, FullPostgresVersionString)

	dbPrivs := []dboperatorv1alpha1.DbPriv{
		{
			Scope: "testdb",
			Privs: "ALL",
		},
	}

	connGetter := func(name string) (*sql.DB, error) {
		return db, nil
	}

	_, err = UpdateUserPrivs(db, "testuser", "CREATEDB", dbPrivs, connGetter)

	if err != nil {
		t.Errorf("unexpected error updating userprivs %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestUpdateSchemaPrivs(t *testing.T) {
	privMap, err := ParsePrivs("USAGE", "testdb.public", &PostgresVersion{ProductName: PostgreSQL})
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}

	expected := map[string]map[string][]string{
		"database": {},
		"table":    {},
		"schema": {
			"testdb.public": []string{"USAGE"},
		},
	}
	if !funk.Equal(privMap, expected) {
		t.Errorf("got %s expected %s", privMap, expected)
	}
}

func addVersionQueryToMock(mock sqlmock.Sqlmock, fullVersionResponse string) {
	expectedVersion := sqlmock.NewRows([]string{"version"})
	expectedVersion.AddRow(fullVersionResponse)
	mock.ExpectQuery("SELECT version();").WillReturnRows(expectedVersion)
}

func expectGetDatabasePrivileges(mock sqlmock.Sqlmock, fullVersionResponse string) {
	expected := sqlmock.NewRows([]string{"datacl"})
	expected.AddRow("{=Tc/postgres,postgres=CTc/postgres,testuser=CTc/postgres}")

	mock.ExpectQuery("SELECT datacl FROM pg_database WHERE datname = $1").WithArgs(
		"testdb",
	).WillReturnRows(expected)
	addVersionQueryToMock(mock, fullVersionResponse)
}

func TestGetDatabasePrivilegest(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	expectGetDatabasePrivileges(mock, FullPostgresVersionString)
	privs, err := GetDatabasePrivileges(db, "testuser", "testdb")
	if err != nil {
		t.Errorf("database privileges failed %s", err)
	}
	expectedPrivs := []string{"CREATE", "TEMPORARY", "CONNECT"}
	if !funk.Equal(privs, expectedPrivs) {
		t.Errorf("got unexpected privileges %s expected %s", privs, expectedPrivs)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestVersionStrParsingPostgres(t *testing.T) {
	result, err := parseVersionString(FullPostgresVersionString)
	if err != nil {
		t.Errorf("parsing version string returned error %s", err)
	}
	expected := PostgresVersion{
		ProductName: PostgreSQL,
		VersionStr:  "15.1",
		Major:       15,
		Minor:       1,
		Patch:       -1,
	}
	if !reflect.DeepEqual(result, &expected) {
		t.Errorf("postgres version parsing returned unexpected result")
	}

}

func TestVersionStrParsingCockroachDb(t *testing.T) {
	result, err := parseVersionString(FullCockroachDBVersionString)
	if err != nil {
		t.Errorf("parsing version string returned error %s", err)
	}
	expected := PostgresVersion{
		ProductName: CockroachDB,
		VersionStr:  "v22.2.2",
		Major:       22,
		Minor:       2,
		Patch:       2,
	}
	if !reflect.DeepEqual(result, &expected) {
		t.Errorf("postgres version parsing returned unexpected result")
	}

}

func XTestGetDefaultPrivilegesFromDb(t *testing.T) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s sslrootcert=%s sslkey=%s sslcert=%s",
		"cockroachdb-public.cockroachdb.svc.cluster.local",
		26257,
		"app1-migration-user",
		"app1-migration-user",
		"app1-db",
		"require",
		"tempCertsDir/host-cockroachdb-public.cockroachdb.svc.cluster.local-user-rootns-kuttl-test-intimate-quagga-secret-cockroachdb-root-cacert1291312790",
		"tempCertsDir/host-cockroachdb-public.cockroachdb.svc.cluster.local-user-rootns-kuttl-test-wondrous-hog-secret-cockroachdb-root-tlskey956724916",
		"host-cockroachdb-public.cockroachdb.svc.cluster.local-user-rootns-kuttl-test-wondrous-hog-secret-cockroachdb-root-tlscert1998637387",
	)
	conn, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Error(err)
		return
	}

	getDefaultPrivileges(conn, "S", "app1-application-user", "app1-migration-user")
}

func TestGetDefaultPrivileges(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	expected := sqlmock.NewRows([]string{"datacl"})
	expected.AddRow("{app1-application-user=adrw/app1-application-user=adrw}")

	mock.ExpectQuery(`SELECT d.defaclacl
	FROM pg_catalog.pg_default_acl d left join pg_catalog.pg_namespace n on n.oid = d.defaclnamespace
	WHERE pg_get_userbyid(d.defaclrole) = $1
	AND n.nspname is NULL
	AND d.defaclobjtype = $2;
	`).WithArgs(
		"app1-migration-user",
		"r",
	).WillReturnRows(expected)

	privs, err := getDefaultPrivileges(db, "r", "app1-application-user", "app1-migration-user")
	if err != nil {
		t.Error(err)
		return
	}

	expectedPrivs := []string{"INSERT", "SELECT", "UPDATE", "DELETE"}
	if !funk.Equal(privs, expectedPrivs) {
		t.Error("Unexpected privileges returned")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
