// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package do_test

import (
	"context"
	"github.com/machbase/neo-spi"
	"sync"
)

// Ensure, that DatabaseMock does implement spi.Database.
// If this is not the case, regenerate this file with moq.
var _ spi.Database = &DatabaseMock{}

// DatabaseMock is a mock implementation of spi.Database.
//
//	func TestSomethingThatUsesDatabase(t *testing.T) {
//
//		// make and configure a mocked spi.Database
//		mockedDatabase := &DatabaseMock{
//			ConnectFunc: func(ctx context.Context, options ...spi.ConnectOption) (spi.Conn, error) {
//				panic("mock out the Connect method")
//			},
//		}
//
//		// use mockedDatabase in code that requires spi.Database
//		// and then make assertions.
//
//	}
type DatabaseMock struct {
	// ConnectFunc mocks the Connect method.
	ConnectFunc func(ctx context.Context, options ...spi.ConnectOption) (spi.Conn, error)

	// calls tracks calls to the methods.
	calls struct {
		// Connect holds details about calls to the Connect method.
		Connect []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Options is the options argument value.
			Options []spi.ConnectOption
		}
	}
	lockConnect sync.RWMutex
}

// Connect calls ConnectFunc.
func (mock *DatabaseMock) Connect(ctx context.Context, options ...spi.ConnectOption) (spi.Conn, error) {
	if mock.ConnectFunc == nil {
		panic("DatabaseMock.ConnectFunc: method is nil but Database.Connect was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Options []spi.ConnectOption
	}{
		Ctx:     ctx,
		Options: options,
	}
	mock.lockConnect.Lock()
	mock.calls.Connect = append(mock.calls.Connect, callInfo)
	mock.lockConnect.Unlock()
	return mock.ConnectFunc(ctx, options...)
}

// ConnectCalls gets all the calls that were made to Connect.
// Check the length with:
//
//	len(mockedDatabase.ConnectCalls())
func (mock *DatabaseMock) ConnectCalls() []struct {
	Ctx     context.Context
	Options []spi.ConnectOption
} {
	var calls []struct {
		Ctx     context.Context
		Options []spi.ConnectOption
	}
	mock.lockConnect.RLock()
	calls = mock.calls.Connect
	mock.lockConnect.RUnlock()
	return calls
}

// Ensure, that ConnMock does implement spi.Conn.
// If this is not the case, regenerate this file with moq.
var _ spi.Conn = &ConnMock{}

// ConnMock is a mock implementation of spi.Conn.
//
//	func TestSomethingThatUsesConn(t *testing.T) {
//
//		// make and configure a mocked spi.Conn
//		mockedConn := &ConnMock{
//			AppenderFunc: func(ctx context.Context, tableName string, opts ...spi.AppenderOption) (spi.Appender, error) {
//				panic("mock out the Appender method")
//			},
//			CloseFunc: func() error {
//				panic("mock out the Close method")
//			},
//			ExecFunc: func(ctx context.Context, sqlText string, params ...any) spi.Result {
//				panic("mock out the Exec method")
//			},
//			QueryFunc: func(ctx context.Context, sqlText string, params ...any) (spi.Rows, error) {
//				panic("mock out the Query method")
//			},
//			QueryRowFunc: func(ctx context.Context, sqlText string, params ...any) spi.Row {
//				panic("mock out the QueryRow method")
//			},
//		}
//
//		// use mockedConn in code that requires spi.Conn
//		// and then make assertions.
//
//	}
type ConnMock struct {
	// AppenderFunc mocks the Appender method.
	AppenderFunc func(ctx context.Context, tableName string, opts ...spi.AppenderOption) (spi.Appender, error)

	// CloseFunc mocks the Close method.
	CloseFunc func() error

	// ExecFunc mocks the Exec method.
	ExecFunc func(ctx context.Context, sqlText string, params ...any) spi.Result

	// QueryFunc mocks the Query method.
	QueryFunc func(ctx context.Context, sqlText string, params ...any) (spi.Rows, error)

	// QueryRowFunc mocks the QueryRow method.
	QueryRowFunc func(ctx context.Context, sqlText string, params ...any) spi.Row

	// calls tracks calls to the methods.
	calls struct {
		// Appender holds details about calls to the Appender method.
		Appender []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// TableName is the tableName argument value.
			TableName string
			// Opts is the opts argument value.
			Opts []spi.AppenderOption
		}
		// Close holds details about calls to the Close method.
		Close []struct {
		}
		// Exec holds details about calls to the Exec method.
		Exec []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// SqlText is the sqlText argument value.
			SqlText string
			// Params is the params argument value.
			Params []any
		}
		// Query holds details about calls to the Query method.
		Query []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// SqlText is the sqlText argument value.
			SqlText string
			// Params is the params argument value.
			Params []any
		}
		// QueryRow holds details about calls to the QueryRow method.
		QueryRow []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// SqlText is the sqlText argument value.
			SqlText string
			// Params is the params argument value.
			Params []any
		}
	}
	lockAppender sync.RWMutex
	lockClose    sync.RWMutex
	lockExec     sync.RWMutex
	lockQuery    sync.RWMutex
	lockQueryRow sync.RWMutex
}

// Appender calls AppenderFunc.
func (mock *ConnMock) Appender(ctx context.Context, tableName string, opts ...spi.AppenderOption) (spi.Appender, error) {
	if mock.AppenderFunc == nil {
		panic("ConnMock.AppenderFunc: method is nil but Conn.Appender was just called")
	}
	callInfo := struct {
		Ctx       context.Context
		TableName string
		Opts      []spi.AppenderOption
	}{
		Ctx:       ctx,
		TableName: tableName,
		Opts:      opts,
	}
	mock.lockAppender.Lock()
	mock.calls.Appender = append(mock.calls.Appender, callInfo)
	mock.lockAppender.Unlock()
	return mock.AppenderFunc(ctx, tableName, opts...)
}

// AppenderCalls gets all the calls that were made to Appender.
// Check the length with:
//
//	len(mockedConn.AppenderCalls())
func (mock *ConnMock) AppenderCalls() []struct {
	Ctx       context.Context
	TableName string
	Opts      []spi.AppenderOption
} {
	var calls []struct {
		Ctx       context.Context
		TableName string
		Opts      []spi.AppenderOption
	}
	mock.lockAppender.RLock()
	calls = mock.calls.Appender
	mock.lockAppender.RUnlock()
	return calls
}

// Close calls CloseFunc.
func (mock *ConnMock) Close() error {
	if mock.CloseFunc == nil {
		panic("ConnMock.CloseFunc: method is nil but Conn.Close was just called")
	}
	callInfo := struct {
	}{}
	mock.lockClose.Lock()
	mock.calls.Close = append(mock.calls.Close, callInfo)
	mock.lockClose.Unlock()
	return mock.CloseFunc()
}

// CloseCalls gets all the calls that were made to Close.
// Check the length with:
//
//	len(mockedConn.CloseCalls())
func (mock *ConnMock) CloseCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockClose.RLock()
	calls = mock.calls.Close
	mock.lockClose.RUnlock()
	return calls
}

// Exec calls ExecFunc.
func (mock *ConnMock) Exec(ctx context.Context, sqlText string, params ...any) spi.Result {
	if mock.ExecFunc == nil {
		panic("ConnMock.ExecFunc: method is nil but Conn.Exec was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		SqlText string
		Params  []any
	}{
		Ctx:     ctx,
		SqlText: sqlText,
		Params:  params,
	}
	mock.lockExec.Lock()
	mock.calls.Exec = append(mock.calls.Exec, callInfo)
	mock.lockExec.Unlock()
	return mock.ExecFunc(ctx, sqlText, params...)
}

// ExecCalls gets all the calls that were made to Exec.
// Check the length with:
//
//	len(mockedConn.ExecCalls())
func (mock *ConnMock) ExecCalls() []struct {
	Ctx     context.Context
	SqlText string
	Params  []any
} {
	var calls []struct {
		Ctx     context.Context
		SqlText string
		Params  []any
	}
	mock.lockExec.RLock()
	calls = mock.calls.Exec
	mock.lockExec.RUnlock()
	return calls
}

// Query calls QueryFunc.
func (mock *ConnMock) Query(ctx context.Context, sqlText string, params ...any) (spi.Rows, error) {
	if mock.QueryFunc == nil {
		panic("ConnMock.QueryFunc: method is nil but Conn.Query was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		SqlText string
		Params  []any
	}{
		Ctx:     ctx,
		SqlText: sqlText,
		Params:  params,
	}
	mock.lockQuery.Lock()
	mock.calls.Query = append(mock.calls.Query, callInfo)
	mock.lockQuery.Unlock()
	return mock.QueryFunc(ctx, sqlText, params...)
}

// QueryCalls gets all the calls that were made to Query.
// Check the length with:
//
//	len(mockedConn.QueryCalls())
func (mock *ConnMock) QueryCalls() []struct {
	Ctx     context.Context
	SqlText string
	Params  []any
} {
	var calls []struct {
		Ctx     context.Context
		SqlText string
		Params  []any
	}
	mock.lockQuery.RLock()
	calls = mock.calls.Query
	mock.lockQuery.RUnlock()
	return calls
}

// QueryRow calls QueryRowFunc.
func (mock *ConnMock) QueryRow(ctx context.Context, sqlText string, params ...any) spi.Row {
	if mock.QueryRowFunc == nil {
		panic("ConnMock.QueryRowFunc: method is nil but Conn.QueryRow was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		SqlText string
		Params  []any
	}{
		Ctx:     ctx,
		SqlText: sqlText,
		Params:  params,
	}
	mock.lockQueryRow.Lock()
	mock.calls.QueryRow = append(mock.calls.QueryRow, callInfo)
	mock.lockQueryRow.Unlock()
	return mock.QueryRowFunc(ctx, sqlText, params...)
}

// QueryRowCalls gets all the calls that were made to QueryRow.
// Check the length with:
//
//	len(mockedConn.QueryRowCalls())
func (mock *ConnMock) QueryRowCalls() []struct {
	Ctx     context.Context
	SqlText string
	Params  []any
} {
	var calls []struct {
		Ctx     context.Context
		SqlText string
		Params  []any
	}
	mock.lockQueryRow.RLock()
	calls = mock.calls.QueryRow
	mock.lockQueryRow.RUnlock()
	return calls
}

// Ensure, that RowMock does implement spi.Row.
// If this is not the case, regenerate this file with moq.
var _ spi.Row = &RowMock{}

// RowMock is a mock implementation of spi.Row.
//
//	func TestSomethingThatUsesRow(t *testing.T) {
//
//		// make and configure a mocked spi.Row
//		mockedRow := &RowMock{
//			ErrFunc: func() error {
//				panic("mock out the Err method")
//			},
//			MessageFunc: func() string {
//				panic("mock out the Message method")
//			},
//			RowsAffectedFunc: func() int64 {
//				panic("mock out the RowsAffected method")
//			},
//			ScanFunc: func(cols ...any) error {
//				panic("mock out the Scan method")
//			},
//			SuccessFunc: func() bool {
//				panic("mock out the Success method")
//			},
//			ValuesFunc: func() []any {
//				panic("mock out the Values method")
//			},
//		}
//
//		// use mockedRow in code that requires spi.Row
//		// and then make assertions.
//
//	}
type RowMock struct {
	// ErrFunc mocks the Err method.
	ErrFunc func() error

	// MessageFunc mocks the Message method.
	MessageFunc func() string

	// RowsAffectedFunc mocks the RowsAffected method.
	RowsAffectedFunc func() int64

	// ScanFunc mocks the Scan method.
	ScanFunc func(cols ...any) error

	// SuccessFunc mocks the Success method.
	SuccessFunc func() bool

	// ValuesFunc mocks the Values method.
	ValuesFunc func() []any

	// calls tracks calls to the methods.
	calls struct {
		// Err holds details about calls to the Err method.
		Err []struct {
		}
		// Message holds details about calls to the Message method.
		Message []struct {
		}
		// RowsAffected holds details about calls to the RowsAffected method.
		RowsAffected []struct {
		}
		// Scan holds details about calls to the Scan method.
		Scan []struct {
			// Cols is the cols argument value.
			Cols []any
		}
		// Success holds details about calls to the Success method.
		Success []struct {
		}
		// Values holds details about calls to the Values method.
		Values []struct {
		}
	}
	lockErr          sync.RWMutex
	lockMessage      sync.RWMutex
	lockRowsAffected sync.RWMutex
	lockScan         sync.RWMutex
	lockSuccess      sync.RWMutex
	lockValues       sync.RWMutex
}

// Err calls ErrFunc.
func (mock *RowMock) Err() error {
	if mock.ErrFunc == nil {
		panic("RowMock.ErrFunc: method is nil but Row.Err was just called")
	}
	callInfo := struct {
	}{}
	mock.lockErr.Lock()
	mock.calls.Err = append(mock.calls.Err, callInfo)
	mock.lockErr.Unlock()
	return mock.ErrFunc()
}

// ErrCalls gets all the calls that were made to Err.
// Check the length with:
//
//	len(mockedRow.ErrCalls())
func (mock *RowMock) ErrCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockErr.RLock()
	calls = mock.calls.Err
	mock.lockErr.RUnlock()
	return calls
}

// Message calls MessageFunc.
func (mock *RowMock) Message() string {
	if mock.MessageFunc == nil {
		panic("RowMock.MessageFunc: method is nil but Row.Message was just called")
	}
	callInfo := struct {
	}{}
	mock.lockMessage.Lock()
	mock.calls.Message = append(mock.calls.Message, callInfo)
	mock.lockMessage.Unlock()
	return mock.MessageFunc()
}

// MessageCalls gets all the calls that were made to Message.
// Check the length with:
//
//	len(mockedRow.MessageCalls())
func (mock *RowMock) MessageCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockMessage.RLock()
	calls = mock.calls.Message
	mock.lockMessage.RUnlock()
	return calls
}

// RowsAffected calls RowsAffectedFunc.
func (mock *RowMock) RowsAffected() int64 {
	if mock.RowsAffectedFunc == nil {
		panic("RowMock.RowsAffectedFunc: method is nil but Row.RowsAffected was just called")
	}
	callInfo := struct {
	}{}
	mock.lockRowsAffected.Lock()
	mock.calls.RowsAffected = append(mock.calls.RowsAffected, callInfo)
	mock.lockRowsAffected.Unlock()
	return mock.RowsAffectedFunc()
}

// RowsAffectedCalls gets all the calls that were made to RowsAffected.
// Check the length with:
//
//	len(mockedRow.RowsAffectedCalls())
func (mock *RowMock) RowsAffectedCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockRowsAffected.RLock()
	calls = mock.calls.RowsAffected
	mock.lockRowsAffected.RUnlock()
	return calls
}

// Scan calls ScanFunc.
func (mock *RowMock) Scan(cols ...any) error {
	if mock.ScanFunc == nil {
		panic("RowMock.ScanFunc: method is nil but Row.Scan was just called")
	}
	callInfo := struct {
		Cols []any
	}{
		Cols: cols,
	}
	mock.lockScan.Lock()
	mock.calls.Scan = append(mock.calls.Scan, callInfo)
	mock.lockScan.Unlock()
	return mock.ScanFunc(cols...)
}

// ScanCalls gets all the calls that were made to Scan.
// Check the length with:
//
//	len(mockedRow.ScanCalls())
func (mock *RowMock) ScanCalls() []struct {
	Cols []any
} {
	var calls []struct {
		Cols []any
	}
	mock.lockScan.RLock()
	calls = mock.calls.Scan
	mock.lockScan.RUnlock()
	return calls
}

// Success calls SuccessFunc.
func (mock *RowMock) Success() bool {
	if mock.SuccessFunc == nil {
		panic("RowMock.SuccessFunc: method is nil but Row.Success was just called")
	}
	callInfo := struct {
	}{}
	mock.lockSuccess.Lock()
	mock.calls.Success = append(mock.calls.Success, callInfo)
	mock.lockSuccess.Unlock()
	return mock.SuccessFunc()
}

// SuccessCalls gets all the calls that were made to Success.
// Check the length with:
//
//	len(mockedRow.SuccessCalls())
func (mock *RowMock) SuccessCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockSuccess.RLock()
	calls = mock.calls.Success
	mock.lockSuccess.RUnlock()
	return calls
}

// Values calls ValuesFunc.
func (mock *RowMock) Values() []any {
	if mock.ValuesFunc == nil {
		panic("RowMock.ValuesFunc: method is nil but Row.Values was just called")
	}
	callInfo := struct {
	}{}
	mock.lockValues.Lock()
	mock.calls.Values = append(mock.calls.Values, callInfo)
	mock.lockValues.Unlock()
	return mock.ValuesFunc()
}

// ValuesCalls gets all the calls that were made to Values.
// Check the length with:
//
//	len(mockedRow.ValuesCalls())
func (mock *RowMock) ValuesCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockValues.RLock()
	calls = mock.calls.Values
	mock.lockValues.RUnlock()
	return calls
}

// Ensure, that ResultMock does implement spi.Result.
// If this is not the case, regenerate this file with moq.
var _ spi.Result = &ResultMock{}

// ResultMock is a mock implementation of spi.Result.
//
//	func TestSomethingThatUsesResult(t *testing.T) {
//
//		// make and configure a mocked spi.Result
//		mockedResult := &ResultMock{
//			ErrFunc: func() error {
//				panic("mock out the Err method")
//			},
//			MessageFunc: func() string {
//				panic("mock out the Message method")
//			},
//			RowsAffectedFunc: func() int64 {
//				panic("mock out the RowsAffected method")
//			},
//		}
//
//		// use mockedResult in code that requires spi.Result
//		// and then make assertions.
//
//	}
type ResultMock struct {
	// ErrFunc mocks the Err method.
	ErrFunc func() error

	// MessageFunc mocks the Message method.
	MessageFunc func() string

	// RowsAffectedFunc mocks the RowsAffected method.
	RowsAffectedFunc func() int64

	// calls tracks calls to the methods.
	calls struct {
		// Err holds details about calls to the Err method.
		Err []struct {
		}
		// Message holds details about calls to the Message method.
		Message []struct {
		}
		// RowsAffected holds details about calls to the RowsAffected method.
		RowsAffected []struct {
		}
	}
	lockErr          sync.RWMutex
	lockMessage      sync.RWMutex
	lockRowsAffected sync.RWMutex
}

// Err calls ErrFunc.
func (mock *ResultMock) Err() error {
	if mock.ErrFunc == nil {
		panic("ResultMock.ErrFunc: method is nil but Result.Err was just called")
	}
	callInfo := struct {
	}{}
	mock.lockErr.Lock()
	mock.calls.Err = append(mock.calls.Err, callInfo)
	mock.lockErr.Unlock()
	return mock.ErrFunc()
}

// ErrCalls gets all the calls that were made to Err.
// Check the length with:
//
//	len(mockedResult.ErrCalls())
func (mock *ResultMock) ErrCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockErr.RLock()
	calls = mock.calls.Err
	mock.lockErr.RUnlock()
	return calls
}

// Message calls MessageFunc.
func (mock *ResultMock) Message() string {
	if mock.MessageFunc == nil {
		panic("ResultMock.MessageFunc: method is nil but Result.Message was just called")
	}
	callInfo := struct {
	}{}
	mock.lockMessage.Lock()
	mock.calls.Message = append(mock.calls.Message, callInfo)
	mock.lockMessage.Unlock()
	return mock.MessageFunc()
}

// MessageCalls gets all the calls that were made to Message.
// Check the length with:
//
//	len(mockedResult.MessageCalls())
func (mock *ResultMock) MessageCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockMessage.RLock()
	calls = mock.calls.Message
	mock.lockMessage.RUnlock()
	return calls
}

// RowsAffected calls RowsAffectedFunc.
func (mock *ResultMock) RowsAffected() int64 {
	if mock.RowsAffectedFunc == nil {
		panic("ResultMock.RowsAffectedFunc: method is nil but Result.RowsAffected was just called")
	}
	callInfo := struct {
	}{}
	mock.lockRowsAffected.Lock()
	mock.calls.RowsAffected = append(mock.calls.RowsAffected, callInfo)
	mock.lockRowsAffected.Unlock()
	return mock.RowsAffectedFunc()
}

// RowsAffectedCalls gets all the calls that were made to RowsAffected.
// Check the length with:
//
//	len(mockedResult.RowsAffectedCalls())
func (mock *ResultMock) RowsAffectedCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockRowsAffected.RLock()
	calls = mock.calls.RowsAffected
	mock.lockRowsAffected.RUnlock()
	return calls
}
