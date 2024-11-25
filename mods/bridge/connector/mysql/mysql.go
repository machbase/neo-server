package mysql

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

func Connect(dsn string) (*sql.DB, error) {
	return sql.Open("mysql", dsn)
}
