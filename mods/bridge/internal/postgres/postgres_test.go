package postgres_test

import (
	"context"
	"os"
	"testing"

	embedded_postgres "github.com/fergusstrange/embedded-postgres"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/bridge/internal"
	"github.com/machbase/neo-server/mods/bridge/internal/postgres"
	"github.com/stretchr/testify/require"
)

var newConn func(context.Context) api.Conn

func TestMain(m *testing.M) {
	conf := embedded_postgres.DefaultConfig().
		Username("dbuser").
		Password("dbpass").
		Database("db").
		Port(15454)
	pgdb := embedded_postgres.NewDatabase(conf)
	if err := pgdb.Start(); err != nil {
		panic(err)
	}
	bridge := postgres.New("pg", "host=127.0.0.1 port=15454 dbname=db user=dbuser password=dbpass sslmode=disable")
	bridge.BeforeRegister()
	defer bridge.AfterUnregister()
	newConn = func(ctx context.Context) api.Conn {
		conn, err := bridge.Connect(ctx)
		if err != nil {
			panic(err)
		}
		return internal.NewConn(conn)
	}
	code := m.Run()
	if err := pgdb.Stop(); err != nil {
		panic(err)
	}
	os.Exit(code)
}

func TestPostgres(t *testing.T) {
	ctx := context.TODO()
	conn := newConn(ctx)
	defer conn.Close()

	conn.Exec(ctx, `CREATE TABLE test (id SERIAL PRIMARY KEY, name TEXT)`)
	conn.Exec(ctx, `INSERT INTO test (name) VALUES ($1)`, "foo")
	conn.Exec(ctx, `INSERT INTO test (name) VALUES ($1)`, "bar")

	rows, err := conn.Query(ctx, `SELECT * FROM test ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		require.NoError(t, rows.Scan(&id, &name))
	}
}
