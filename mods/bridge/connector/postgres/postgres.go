package postgres

import (
	"database/sql"

	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}
