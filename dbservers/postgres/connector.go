package postgres

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/obeleh/db-operator/shared"
)

type PostgresConnector struct {
}

func (c *PostgresConnector) Connect(connectInfo *shared.DbServerConnectInfo, credentials *shared.Credentials, databaseName *string) (*sql.DB, error) {
	dbName := connectInfo.Database
	if dbName == "" {
		dbName = "postgres"
	}
	if databaseName != nil {
		dbName = *databaseName
	}

	// https://www.postgresql.org/docs/current/libpq-ssl.html#LIBPQ-SSL-PROTECTION
	sslMode, found := connectInfo.Options["sslmode"]
	if !found {
		sslMode = "require"
	}

	var connStr string
	var err error
	if credentials == nil {
		connStr, err = getConnectionString(
			connectInfo.Host,
			connectInfo.UserName,
			dbName,
			sslMode,
			connectInfo.Port,
			connectInfo.Password,
			connectInfo.CaCert,
			connectInfo.TlsCrt,
			connectInfo.TlsKey,
		)
	} else {
		var caCert *string
		if credentials.CaCert != nil {
			caCert = credentials.CaCert
		} else if connectInfo.CaCert != nil {
			caCert = connectInfo.CaCert
		}
		connStr, err = getConnectionString(
			connectInfo.Host,
			credentials.UserName,
			dbName,
			sslMode,
			connectInfo.Port,
			credentials.Password,
			caCert,
			credentials.TlsCrt,
			credentials.TlsKey,
		)
	}

	if err != nil {
		return nil, err
	}

	return sql.Open("postgres", connStr)
}

func getConnectionString(host, userName, dbName, sslMode string, port int, password, caCert, tlsCrt, tlsKey *string) (string, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=%s",
		host, port, userName, dbName, sslMode)

	if password != nil {
		connStr += fmt.Sprintf(" password=%s", *password)
	}

	// For some reason it's not possible yet to load Tls Certs from memory so we write to file
	// Open PR: https://github.com/lib/pq/pull/1066/files
	if caCert != nil || tlsCrt != nil || tlsKey != nil {
		tempCertsDir := filepath.Join(".", "tempCertsDir")
		_ = os.MkdirAll(tempCertsDir, os.ModePerm)

		if caCert != nil {
			filePath, err := writeToTempFile(*caCert)
			if err != nil {
				return "", err
			}
			connStr += fmt.Sprintf(" sslrootcert=%s", filePath)
		}

		if tlsKey != nil {
			filePath, err := writeToTempFile(*tlsKey)
			if err != nil {
				return "", err
			}
			connStr += fmt.Sprintf(" sslkey=%s", filePath)
		}

		if tlsCrt != nil {
			filePath, err := writeToTempFile(*tlsCrt)
			if err != nil {
				return "", err
			}
			connStr += fmt.Sprintf(" sslcert=%s", filePath)
		}
	}
	return connStr, nil
}

func writeToTempFile(contents string) (string, error) {
	byteContent := []byte(contents)
	sha256Sum := sha256.Sum256(byteContent)
	fileName := fmt.Sprintf("%x", sha256Sum)

	filePath := filepath.Join(".", "tempCertsDir", fileName)
	_, err := os.Stat(filePath)
	if err == nil {
		return filePath, nil // file existed
	}
	return filePath, os.WriteFile(filePath, byteContent, 0600)
}
