package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge/internal"
	bridgePostgres "github.com/machbase/neo-server/v8/mods/bridge/internal/postgres"
	"github.com/ory/dockertest/v4"
	"github.com/stretchr/testify/require"
)

func TestPostgres(t *testing.T) {
	pool := dockertest.NewPoolT(t, "")
	postgres := pool.RunT(t, "postgres",
		dockertest.WithTag("16.13"),
		dockertest.WithEnv([]string{
			"POSTGRES_USER=dbuser",
			"POSTGRES_PASSWORD=dbpass",
			"POSTGRES_DB=db",
		}),
	)
	hostPort := postgres.GetHostPort("5432/tcp")
	host, port, _ := net.SplitHostPort(hostPort)
	dsn := fmt.Sprintf("host=%s port=%s dbname=db user=dbuser password=dbpass sslmode=disable", host, port)
	// wait for postgres to be ready
	err := pool.Retry(t.Context(), 30*time.Second, func() error {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return err
		}
		return db.Ping()
	})
	if err != nil {
		t.Fatalf("could not connect to postgres: %v", err)
	}

	bridge := bridgePostgres.New("pg", dsn)
	bridge.BeforeRegister()
	defer bridge.AfterUnregister()

	newConn := func(ctx context.Context) api.Conn {
		conn, err := bridge.Connect(ctx)
		if err != nil {
			panic(err)
		}
		return internal.NewConn(conn)
	}

	ctx := t.Context()
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
