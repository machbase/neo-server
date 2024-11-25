package mssql

import (
	"database/sql"

	_ "github.com/microsoft/go-mssqldb"
)

func Connect(dsn string) (*sql.DB, error) {
	return sql.Open("sqlserver", dsn)
}
