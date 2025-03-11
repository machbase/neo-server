package tql

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPragma2(t *testing.T) {
	tests := []struct {
		Name       string
		Script     string
		ExpectFunc func(t *testing.T, task *Task)
	}{
		{
			Name: "pragma-log-level-bang",
			Script: `
				#pragma log-level=warn
				FAKE( linspace(1, 5, 5))
				JSON()`,
			ExpectFunc: func(t *testing.T, task *Task) {
				require.Equal(t, ParseLogLevel("warn"), task.logLevel)
			},
		},
		{
			Name: "pragma-log-level",
			Script: `
				//+ log-level=trace sql-thread-lock
				SQL( 'select count(*) from example' )
				JSON()`,
			ExpectFunc: func(t *testing.T, task *Task) {
				require.Equal(t, ParseLogLevel("trace"), task.logLevel)
				require.Equal(t, true, task.nodes[0].PragmaBool(PRAGMA_SQL_THREAD_LOCK))
			},
		},
		{
			Name: "pragma-sql-thread-lock-bang",
			Script: `
				#pragma sql-thread-lock=0
				SQL( 'select count(*) from example' )
				JSON()`,
			ExpectFunc: func(t *testing.T, task *Task) {
				require.Equal(t, ParseLogLevel("error"), task.logLevel)
				require.Equal(t, false, task.nodes[0].PragmaBool(PRAGMA_SQL_THREAD_LOCK))
			},
		},
	}
	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			task := NewTaskContext(ctx)
			task.SetLogWriter(os.Stdout)
			if err := task.CompileString(tc.Script); err != nil {
				t.Log("ERROR:", tc.Name, err.Error())
				t.Fail()
				return
			}
			tc.ExpectFunc(t, task)
		})
	}
}
