package mysql

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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

/*func TestPrivilegesUnpack(t *testing.T) {
	example := "mydb.*:INSERT,UPDATE/anotherdb.*:SELECT(col1,col2),UPDATE/yetanother.*:ALL"
	privilegesUnpack(example, "")
}*/

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

func TestPointer(t *testing.T) {
	var a *int
	for n, str := range []string{"a", "b", "c"} {
		if str == "b" {
			curN := n
			a = &curN
			fmt.Printf("now %d", n)
		}
	}

	if *a != 1 {
		t.Fatalf("pointer behaviour not like I understood it %d", *a)
	}
}
