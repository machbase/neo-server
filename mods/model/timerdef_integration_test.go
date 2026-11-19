//go:build integration

package model_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/stretchr/testify/require"
)

func TestTimerDefinitionMachbaseUpgrade(t *testing.T) {
	tmpRoot := filepath.Join("..", "..", "tmp")
	require.NoError(t, os.MkdirAll(tmpRoot, 0755))
	dir, err := os.MkdirTemp(tmpRoot, "timer-schema-")
	require.NoError(t, err)
	server := &machsvr.TestServer{}
	server.StartServer(dir)
	t.Cleanup(server.StopServer)
	ctx := context.Background()
	conn, err := spi.Connect(ctx, "sys")
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `CREATE TABLE _NEO_TIMER_DEF (
ID LONG PRIMARY KEY AUTO_INCREMENT, USER_NAME VARCHAR(64) NOT NULL,
NAME VARCHAR(40) NOT NULL, EXEC_USER VARCHAR(64) NOT NULL, AUTO_START SHORT NOT NULL,
DISABLED SHORT NOT NULL, LAST_ERROR VARCHAR(1024), LAST_ERROR_AT DATETIME,
TASK VARCHAR(4096) NOT NULL, SCHEDULE VARCHAR(128) NOT NULL, ATTRIBUTES JSON)`)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `INSERT INTO _NEO_TIMER_DEF
(USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, TASK, SCHEDULE)
VALUES ('SYS', 'OLD', 'SYS', 1, 0, 'old error', 'old.tql', '@every 1m')`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	provider := model.NewProvider()
	rows, err := provider.LoadAllTimers(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.True(t, rows[0].ContinueOnError)
	require.True(t, rows[0].AutoStart)
	require.Equal(t, "old error", rows[0].LastError)
	require.Equal(t, "old.tql", rows[0].Task)

	rows, err = model.NewProvider().LoadAllTimers(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.True(t, rows[0].ContinueOnError)

	row := rows[0]
	row.ContinueOnError = false
	require.NoError(t, provider.SaveTimer(ctx, row))
	require.NoError(t, provider.SetTimerRuntimeError(ctx, row.Id, "new error"))
	row.Task = "new.tql"
	require.NoError(t, provider.SaveTimer(ctx, row))
	rows, err = model.NewProvider().LoadAllTimers(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.False(t, rows[0].ContinueOnError)
	require.Equal(t, "new error", rows[0].LastError)
	require.False(t, rows[0].LastErrorAt.IsZero())
	require.Equal(t, "new.tql", rows[0].Task)
}
