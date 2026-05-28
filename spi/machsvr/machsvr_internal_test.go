package machsvr

import (
	"context"
	"testing"

	"github.com/machbase/neo-client/api"
	"github.com/stretchr/testify/require"
)

func TestConnCancelNilHandle(t *testing.T) {
	conn := &Conn{db: &Database{}}
	require.Error(t, conn.Cancel())
}

func TestConnCloseNilHandle(t *testing.T) {
	conn := &Conn{db: &Database{}}
	require.ErrorIs(t, conn.Close(), api.ErrDatabaseNoConnection)
}

func TestConnCloseSignalsReturnChan(t *testing.T) {
	conn, err := _env.database.Connect(context.Background(), api.WithPassword("sys", "manager"))
	require.NoError(t, err)

	machConn, ok := conn.(*Conn)
	require.True(t, ok)

	machConn.returnChan = make(chan struct{}, 1)
	require.NoError(t, machConn.Close())

	select {
	case <-machConn.returnChan:
	default:
		t.Fatal("expected Close to signal returnChan")
	}

	require.NoError(t, machConn.Close())
}
