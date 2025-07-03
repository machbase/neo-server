//go:build arm
// +build arm

package duckdb

import (
	"database/sql"
	"fmt"
)

func Connect(dsn string) (*sql.DB, error) {
	return nil, fmt.Errorf("DuckDB is not supported on ARM architecture")
}
