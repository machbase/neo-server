//go:build !arm || !386
// +build !arm !386

package duckdb

import (
	"context"
	"database/sql"

	"github.com/marcboeker/go-duckdb"
)

func Connect(dsn string) (*sql.DB, error) {
	connector, err := duckdb.NewConnector("", nil)
	if err != nil {
		return nil, err
	}
	_, err = connector.Connect(context.Background())
	return nil, err
}
