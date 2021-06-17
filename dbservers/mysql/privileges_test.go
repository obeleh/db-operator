package mysql

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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
