//go:build integration
// +build integration

package mysql

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
)

func TestIntegrationPrivilegesGrant(t *testing.T) {
	serverConn, err := sql.Open("mysql", "root:mysqlPassword@tcp(127.0.0.1:3306)/")
	if err != nil {
		t.Fatalf("sql.Open failed: %s", err)
	}
	defer serverConn.Close()
	_, err = serverConn.Exec("CREATE DATABASE IF NOT EXISTS kitchen;")
	if err != nil {
		t.Fatalf("failed creating database: %s", err)
	}

	db, err := sql.Open("mysql", "root:mysqlPassword@tcp(127.0.0.1:3306)/kitchen")
	if err != nil {
		t.Fatalf("sql.Open failed: %s", err)
	}
	defer db.Close()

	tlsReq := TlsRequires{}
	si, err := getServerInfo(db)
	if err != nil {
		t.Errorf("NewServerInfo failed: %s", err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS chair (name VARCHAR(20));")

	err = privilegesGrant(db, "jantje", "%", "chair", []string{"SELECT"}, tlsReq, *si)
	if err != nil {
		t.Errorf("privilegesGrant failed: %s", err)
	}

	serverGrants, err := getServerGrants(db, "jantje", "%")
	if err != nil {
		t.Errorf("getGrants failed: %s", err)
	}

	if !reflect.DeepEqual(serverGrants, []string{"USAGE"}) {
		t.Error("Expexted Usage in server grants")
	}

	privs, err := getPrivileges(db, "jantje", "%")
	if err != nil {
		t.Errorf("getPrivileges failed: %s", err)
	}
	expected := map[string][]string{
		"\"kitchen\".\"chair\"": {"SELECT"},
		"*.*":                   {"USAGE"},
	}
	for ky, vl := range expected {
		if !reflect.DeepEqual(privs[ky], expected[ky]) {
			t.Errorf(fmt.Sprintf("Expected privs for %s to be %s, got %s", ky, vl, privs[ky]))
		}
	}

	_, err = serverConn.Exec("DROP DATABASE IF EXISTS kitchen;")
	if err != nil {
		t.Fatalf("failed dropping database: %s", err)
	}
}

func TestIntegrationUpdateUserPrivs(t *testing.T) {

	// UpdateUserPrivs(conn *sql.DB, userName string, serverPrivs string, dbPrivs []dboperatorv1alpha1.DbPriv)

	serverConn, err := sql.Open("mysql", "root:mysqlPassword@tcp(127.0.0.1:3306)/")
	if err != nil {
		t.Fatalf("sql.Open failed: %s", err)
	}
	defer serverConn.Close()
	_, err = serverConn.Exec("CREATE DATABASE IF NOT EXISTS kitchen;")
	if err != nil {
		t.Fatalf("failed creating database: %s", err)
	}

	db, err := sql.Open("mysql", "root:mysqlPassword@tcp(127.0.0.1:3306)/kitchen")
	if err != nil {
		t.Fatalf("sql.Open failed: %s", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS chair (name VARCHAR(20));")
	dbPrivs := []dboperatorv1alpha1.DbPriv{
		{
			DbName: "kitchen.chair",
			Privs:  "SELECT,UPDATE",
		},
	}
	changes, err := UpdateUserPrivs(db, "jantje", "TODO", dbPrivs)
	if err != nil {
		t.Errorf("UpdateUserPrivs failed: %s", err)
	}

	if !changes {
		t.Error("Expected changes")
	}
}
