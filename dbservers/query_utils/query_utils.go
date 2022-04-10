package query_utils

import (
	"database/sql"
	"fmt"
	"strings"
)

func SelectFirstValueString(conn *sql.DB, query string, args ...interface{}) (string, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return "", err
	}

	if rows.Next() {
		value := sql.NullString{}
		err := rows.Scan(&value)
		if err != nil {
			return "", fmt.Errorf("unable to load string")
		}
		if !value.Valid {
			return "", fmt.Errorf("got null, expected string")
		}
		return value.String, nil
	} else {
		return "", fmt.Errorf("no rows returned")
	}
}

func SelectFirstValueStringNullToEmpty(conn *sql.DB, query string, args ...interface{}) (string, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return "", err
	}

	if rows.Next() {
		value := sql.NullString{}
		err := rows.Scan(&value)
		if err != nil {
			return "", fmt.Errorf("unable to load string")
		}
		if !value.Valid {
			return "", nil
		}
		return value.String, nil
	} else {
		return "", fmt.Errorf("no rows returned")
	}
}

func SelectFirstValueInt(conn *sql.DB, query string, args ...interface{}) (int, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return -1, err
	}

	if rows.Next() {
		value := sql.NullInt32{}
		err := rows.Scan(&value)
		if err != nil {
			return -1, fmt.Errorf("unable to load int")
		}
		if !value.Valid {
			return -1, fmt.Errorf("got null, expected int")
		}
		return int(value.Int32), nil
	} else {
		return -1, fmt.Errorf("no rows returned")
	}
}

func SelectFirstValueStringSlice(conn *sql.DB, query string, args ...interface{}) ([]string, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	output := []string{}
	for rows.Next() {
		value := sql.NullString{}
		err := rows.Scan(&value)
		if err != nil {
			return nil, fmt.Errorf("unable to load string")
		}
		if !value.Valid {
			return nil, fmt.Errorf("got null, expected string")
		}
		output = append(output, value.String)
	}

	return output, nil
}

func GetStringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	s += len(start)
	e := strings.Index(str[s:], end)
	if e == -1 {
		return
	}
	return str[s : e+s]
}
