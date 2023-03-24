package postgres

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/obeleh/db-operator/shared"
)

type PostgresConnector struct {
}

func (c *PostgresConnector) Connect(connectInfo *shared.DbServerConnectInfo) (*sql.DB, error) {
	dbName := connectInfo.Database
	if dbName == "" {
		dbName = "postgres"
	}

	// https://www.postgresql.org/docs/current/libpq-ssl.html#LIBPQ-SSL-PROTECTION
	sslMode, found := connectInfo.Options["sslmode"]
	if !found {
		sslMode = "require"
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=%s",
		connectInfo.Host, connectInfo.Port, connectInfo.UserName, dbName, sslMode)

	if connectInfo.Password != nil {
		connStr += fmt.Sprintf(" password=%s", *connectInfo.Password)
	}

	// For some reason it's not possible yet to load Tls Certs from memory so we write to file
	// Open PR: https://github.com/lib/pq/pull/1066/files

	if connectInfo.CaCert != nil || connectInfo.TlsCrt != nil || connectInfo.TlsKey != nil {
		prefixStr := fmt.Sprintf("host-%s-user-%s", connectInfo.Host, connectInfo.UserName)
		if connectInfo.Credentials.SourceSecret != nil {
			prefixStr = fmt.Sprintf("%sns-%s-secret-%s", prefixStr, connectInfo.Credentials.SourceSecret.Namespace, connectInfo.Credentials.SourceSecret.Name)
		}

		tempCertsDir := filepath.Join(".", "tempCertsDir")
		_ = os.MkdirAll("tempCertsDir", os.ModePerm)

		if connectInfo.CaCert != nil {
			cacertFile, _ := ioutil.TempFile(tempCertsDir, prefixStr+"-cacert")
			cacertFile.WriteString(*connectInfo.CaCert)
			connStr += fmt.Sprintf(" sslrootcert=%s", cacertFile.Name())
		}

		if connectInfo.TlsKey != nil {
			tlsKeyFile, _ := ioutil.TempFile(tempCertsDir, prefixStr+"-tlskey")
			tlsKeyFile.WriteString(*connectInfo.TlsKey)
			connStr += fmt.Sprintf(" sslkey=%s", tlsKeyFile.Name())
		}

		if connectInfo.TlsCrt != nil {
			tlsCrtFile, _ := ioutil.TempFile(tempCertsDir, prefixStr+"-tlscert")
			tlsCrtFile.WriteString(*connectInfo.TlsCrt)
			connStr += fmt.Sprintf(" sslcert=%s", tlsCrtFile.Name())
		}
	}

	return sql.Open("postgres", connStr)
}
