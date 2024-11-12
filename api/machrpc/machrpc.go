// package machrpc is a reference implementation of client
// that interwork with machbase-neo server via gRPC.
package machrpc

import (
	context "context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	ServerAddr    string
	Tls           *TlsConfig
	QueryTimeout  time.Duration
	AppendTimeout time.Duration
	ConnProvider  func() (*grpc.ClientConn, error)
}

type TlsConfig struct {
	ClientCert string
	ClientKey  string
	ServerCert string
}

// Client is a convenient data type represents client side of machbase-neo.
//
//	client := machrpc.NewClient(WithServer(serverAddr, "path/to/server_cert.pem"))
//
// serverAddr can be tcp://ip_addr:port or unix://path.
// The path of unix domain socket can be absolute/relative path.
type Client struct {
	grpcConn grpc.ClientConnInterface
	cli      MachbaseClient

	serverAddr    string
	serverCert    string
	certPath      string
	keyPath       string
	queryTimeout  time.Duration
	appendTimeout time.Duration

	closeOnce sync.Once
}

var _ api.Database = (*Client)(nil)

// NewClient creates new instance of Client.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	client := &Client{
		serverAddr:    cfg.ServerAddr,
		queryTimeout:  cfg.QueryTimeout,
		appendTimeout: cfg.AppendTimeout,
	}
	if cfg.Tls != nil {
		client.certPath = cfg.Tls.ClientCert
		client.keyPath = cfg.Tls.ClientKey
		client.serverCert = cfg.Tls.ServerCert
	}

	var conn *grpc.ClientConn
	var err error

	if cfg.ConnProvider != nil {
		conn, err = cfg.ConnProvider()
	} else if client.serverAddr != "" {
		if client.keyPath != "" && client.certPath != "" && client.serverCert != "" {
			conn, err = MakeGrpcTlsConn(client.serverAddr, client.keyPath, client.certPath, client.serverCert)
		} else {
			conn, err = MakeGrpcConn(client.serverAddr, nil)
		}
	} else {
		return nil, errors.New("server address is not specified")
	}
	if err != nil {
		return nil, errors.Wrap(err, "NewClient")
	}
	client.grpcConn = conn
	client.cli = NewMachbaseClient(conn)

	return client, nil
}

func NewClientWithRPCClient(cli MachbaseClient) *Client {
	return &Client{
		cli: cli,
	}
}

func MakeGrpcInsecureConn(addr string) (grpc.ClientConnInterface, error) {
	return MakeGrpcConn(addr, nil)
}

func MakeGrpcTlsConn(addr string, keyPath string, certPath string, caCertPath string) (*grpc.ClientConn, error) {
	cert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(cert) {
		return nil, fmt.Errorf("fail to load server CA cert")
	}

	tlsCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		RootCAs:            certPool,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}

	return MakeGrpcConn(addr, tlsConfig)
}

func MakeGrpcConn(addr string, tlsConfig *tls.Config) (*grpc.ClientConn, error) {
	pwd, _ := os.Getwd()
	if strings.HasPrefix(addr, "unix://../") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(filepath.Dir(pwd), addr[len("unix://../"):]))
	} else if strings.HasPrefix(addr, "../") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(filepath.Dir(pwd), addr[len("../"):]))
	} else if strings.HasPrefix(addr, "unix://./") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(pwd, addr[len("unix://./"):]))
	} else if strings.HasPrefix(addr, "./") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(pwd, addr[len("./"):]))
	} else if strings.HasPrefix(addr, "/") {
		addr = fmt.Sprintf("unix://%s", addr)
	} else {
		addr = strings.TrimPrefix(addr, "http://")
		addr = strings.TrimPrefix(addr, "tcp://")
	}

	if tlsConfig == nil || strings.HasPrefix(addr, "unix://") {
		return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		return grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}
}

func (client *Client) Close() {
	client.closeOnce.Do(func() {
		client.grpcConn = nil
		client.cli = nil
	})
}

func (client *Client) UserAuth(ctx context.Context, user string, password string) (bool, string, error) {
	req := &UserAuthRequest{LoginName: user, Password: password}
	rsp, err := client.cli.UserAuth(ctx, req)
	if err != nil {
		return false, "", err
	}
	if !rsp.Success {
		return false, rsp.Reason, nil
	}
	return true, "", nil
}

func (client *Client) Ping(ctx context.Context) (time.Duration, error) {
	sent := time.Now().UnixNano()
	req := &PingRequest{Token: sent}
	rsp, err := client.cli.Ping(ctx, req)
	if err != nil {
		return 0, err
	}
	if sent != rsp.Token {
		return 0, fmt.Errorf("invalid token %d != %d", sent, rsp.Token)
	}
	return time.Duration(time.Now().UnixNano() - sent), nil
}

// Connect make a connection to the server
func (client *Client) Connect(ctx context.Context, opts ...api.ConnectOption) (api.Conn, error) {
	ret := &Conn{client: client}
	req := &ConnRequest{
		User:     ret.dbUser,
		Password: ret.dbPassword,
	}
	for _, o := range opts {
		switch v := o.(type) {
		case *api.ConnectOptionPassword:
			req.User = v.User
			req.Password = v.Password
		case *api.ConnectOptionTrustUser:
			return nil, errors.New("trust user option is not supported")
		default:
			return nil, fmt.Errorf("unknown option type-%T", o)
		}
	}

	if req.User == "" {
		return nil, errors.New("no user specified, use WithPassword() option")
	}
	rsp, err := client.cli.Conn(ctx, req)
	if err != nil {
		return nil, err
	}

	if !rsp.Success {
		return nil, errors.New(rsp.Reason)
	}
	ret.ctx = ctx
	ret.handle = rsp.Conn
	return ret, nil
}

type Conn struct {
	ctx    context.Context
	client *Client

	dbUser     string
	dbPassword string

	handle    *ConnHandle
	closeOnce sync.Once
}

var _ api.Conn = (*Conn)(nil)

func (conn *Conn) Close() error {
	var err error
	conn.closeOnce.Do(func() {
		req := &ConnCloseRequest{Conn: conn.handle}
		_, err = conn.client.cli.ConnClose(conn.ctx, req)
	})
	return err
}

// Explain retrieve execution plan of the given SQL statement.
func (conn *Conn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
	req := &ExplainRequest{Conn: conn.handle, Sql: sqlText, Full: full}
	rsp, err := conn.client.cli.Explain(ctx, req)
	if err != nil {
		return "", err
	}
	if !rsp.Success {
		return "", fmt.Errorf(rsp.Reason)
	}
	return rsp.Plan, nil
}

// Exec executes SQL statements that does not return result
// like 'ALTER', 'CREATE TABLE', 'DROP TABLE', ...
func (conn *Conn) Exec(ctx context.Context, sqlText string, params ...any) api.Result {
	pbParams, err := ConvertAnyToPb(params)
	if err != nil {
		return &Result{err: err}
	}
	req := &ExecRequest{Conn: conn.handle, Sql: sqlText, Params: pbParams}
	rsp, err := conn.client.cli.Exec(ctx, req)
	if err != nil {
		return &Result{err: err}
	}
	if !rsp.Success {
		return &Result{err: errors.New(rsp.Reason), message: rsp.Reason}
	}
	return &Result{message: rsp.Reason, rowsAffected: rsp.RowsAffected}
}

type Result struct {
	err          error
	rowsAffected int64
	message      string
}

func (r *Result) Err() error {
	return r.err
}

func (r *Result) RowsAffected() int64 {
	return r.rowsAffected
}

func (r *Result) Message() string {
	return r.message
}

// Query executes SQL statements that are expected multiple rows as result.
// Commonly used to execute 'SELECT * FROM <TABLE>'
//
// Rows returned by QueryContext() must be closed to prevent leaking resources.
//
//	ctx, cancelFunc := context.WithTimeout(5*time.Second)
//	defer cancelFunc()
//
//	rows, err := client.Query(ctx, "select * from my_table where name = ?", my_name)
//	if err != nil {
//		panic(err)
//	}
//	defer rows.Close()
func (conn *Conn) Query(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	pbParams, err := ConvertAnyToPb(params)
	if err != nil {
		return nil, err
	}

	req := &QueryRequest{Conn: conn.handle, Sql: sqlText, Params: pbParams}
	rsp, err := conn.client.cli.Query(ctx, req)
	if err != nil {
		return nil, err
	}

	if rsp.Success {
		return &Rows{
			ctx:          ctx,
			client:       conn.client,
			rowsAffected: rsp.RowsAffected,
			message:      rsp.Reason,
			handle:       rsp.RowsHandle,
		}, nil
	} else {
		if len(rsp.Reason) > 0 {
			return nil, errors.New(rsp.Reason)
		}
		return nil, errors.New("unknown error")
	}
}

type Rows struct {
	ctx          context.Context
	client       *Client
	message      string
	rowsAffected int64
	handle       *RowsHandle
	values       []any
	err          error
	closeOnce    sync.Once
}

// Close release all resources that assigned to the Rows
func (rows *Rows) Close() error {
	var err error
	rows.closeOnce.Do(func() {
		_, err = rows.client.cli.RowsClose(rows.ctx, rows.handle)
	})
	return err
}

// IsFetchable returns true if statement that produced this Rows was fetch-able (e.g was select?)
func (rows *Rows) IsFetchable() bool {
	return rows.handle != nil
}

func (rows *Rows) Message() string {
	return rows.message
}

func (rows *Rows) RowsAffected() int64 {
	return rows.rowsAffected
}

// Columns returns list of column info that consists of result of query statement.
func (rows *Rows) Columns() (api.Columns, error) {
	rsp, err := rows.client.cli.Columns(rows.ctx, rows.handle)
	if err != nil {
		return nil, err
	}
	ret := make(api.Columns, len(rsp.Columns))
	if rsp.Success {
		for i, c := range rsp.Columns {
			ret[i] = &api.Column{
				Name:     c.Name,
				Type:     api.ColumnType(c.Type),
				Length:   int(c.Length),
				DataType: api.DataType(c.DataType),
				Flag:     api.ColumnFlag(c.Flag),
			}
		}
		return ret, nil
	} else {
		if len(rsp.Reason) > 0 {
			return nil, errors.New(rsp.Reason)
		} else {
			return nil, fmt.Errorf("fail to get columns info")
		}
	}
}

// Next returns true if there are at least one more record that can be fetchable
// rows, _ := client.Query("select name, value from my_table")
//
//	for rows.Next(){
//		var name string
//		var value float64
//		rows.Scan(&name, &value)
//	}
func (rows *Rows) Next() bool {
	if rows.err != nil {
		return false
	}
	rsp, err := rows.client.cli.RowsFetch(rows.ctx, rows.handle)
	if err != nil {
		rows.err = err
		return false
	}
	if rsp.Success {
		if rsp.HasNoRows {
			return false
		}
		rows.values = ConvertPbToAny(rsp.Values)
	} else {
		if len(rsp.Reason) > 0 {
			rows.err = errors.New(rsp.Reason)
		}
		rows.values = nil
	}
	return !rsp.HasNoRows
}

// Scan retrieve values of columns
//
//	for rows.Next(){
//		var name string
//		var value float64
//		rows.Scan(&name, &value)
//	}
func (rows *Rows) Scan(cols ...any) error {
	if rows.err != nil {
		return rows.err
	}
	if rows.values == nil {
		return sql.ErrNoRows
	}
	if len(rows.values) < len(cols) {
		return fmt.Errorf("column count mismatch %d != %d", len(rows.values), len(cols))
	}
	return scan(rows.values, cols)
}

// QueryRow executes a SQL statement that expects a single row result.
//
//	var cnt int
//	row := client.QueryRow(ctx, "select count(*) from my_table where name = ?", "my_name")
//	row.Scan(&cnt)
func (conn *Conn) QueryRow(ctx context.Context, sqlText string, params ...any) api.Row {
	pbParams, err := ConvertAnyToPb(params)
	if err != nil {
		return &Row{success: false, err: err}
	}

	req := &QueryRowRequest{Conn: conn.handle, Sql: sqlText, Params: pbParams}
	rsp, err := conn.client.cli.QueryRow(ctx, req)
	if err != nil {
		return &Row{success: false, err: err}
	}

	var row = &Row{}
	row.success = rsp.Success
	row.rowsAffected = rsp.RowsAffected
	row.message = rsp.Reason
	row.err = nil
	if !rsp.Success && len(rsp.Reason) > 0 {
		row.err = errors.New(rsp.Reason)
	}
	row.values = ConvertPbToAny(rsp.Values)
	row.columns = make(api.Columns, len(row.values))
	for i, c := range rsp.Columns {
		row.columns[i] = &api.Column{
			Name:     c.Name,
			Type:     api.ColumnType(c.Type),
			Length:   int(c.Length),
			DataType: api.ParseDataType(c.DataType),
			Flag:     api.ColumnFlag(c.Flag),
		}
	}
	return row
}

type Row struct {
	success      bool
	err          error
	rowsAffected int64
	message      string
	columns      api.Columns
	values       []any
}

func (row *Row) Success() bool {
	return row.success
}

func (row *Row) Err() error {
	return row.err
}

func (row *Row) Scan(cols ...any) error {
	if row.err != nil {
		return row.err
	}
	if !row.success {
		return sql.ErrNoRows
	}
	err := scan(row.values, cols)
	return err
}

func (row *Row) Columns() (api.Columns, error) {
	if row.err != nil {
		return nil, row.err
	}
	return row.columns, nil
}

func (row *Row) RowsAffected() int64 {
	return row.rowsAffected
}

func (row *Row) Message() string {
	return row.message
}

func (row *Row) Values() []any {
	return row.values
}

func scan(src []any, dst []any) error {
	for i := range dst {
		if i >= len(src) {
			return fmt.Errorf("column %d is out of range %d", i, len(src))
		}
		if src[i] == nil {
			dst[i] = nil
			continue
		}
		if err := api.Scan(src[i], dst[i]); err != nil {
			return err
		}
	}
	return nil
}

// Appender creates a new Appender for the given table.
// Appender should be closed otherwise it may cause server side resource leak.
//
//	app, _ := client.Appender(ctx, "MY_TABLE")
//	defer app.Close()
//	app.Append("name", time.Now(), 3.14)
func (conn *Conn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {

	ap := &Appender{
		ctx:             ctx,
		bufferThreshold: 400,
	}

	for _, opt := range opts {
		switch v := opt.(type) {
		case *api.AppenderOptionBuffer:
			ap.bufferThreshold = v.Threshold
		default:
			return nil, fmt.Errorf("unknown option type-%T", opt)
		}
	}

	openRsp, err := conn.client.cli.Appender(ctx, &AppenderRequest{
		Conn:      conn.handle,
		TableName: tableName,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Appender")
	}

	if !openRsp.Success {
		return nil, errors.New(openRsp.Reason)
	}

	for _, c := range openRsp.Columns {
		col := &api.Column{
			Name:     c.Name,
			Type:     api.ColumnType(c.Type),
			Length:   int(c.Length),
			DataType: api.DataType(c.DataType),
			Flag:     api.ColumnFlag(c.Flag),
		}
		ap.columns = append(ap.columns, col)
	}

	appendClient, err := conn.client.cli.Append(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "AppendClient")
	}

	ap.client = conn.client
	ap.appendClient = appendClient
	ap.tableName = openRsp.TableName
	ap.tableType = api.TableType(openRsp.TableType)
	ap.handle = openRsp.Handle

	ap.bufferTicker = time.NewTicker(time.Second)
	go func() {
		for range ap.bufferTicker.C {
			ap.flush(nil)
		}
		ap.bufferTickerDone.Done()
	}()

	return ap, nil
}

type Appender struct {
	ctx          context.Context
	client       *Client
	appendClient Machbase_AppendClient
	tableName    string
	tableType    api.TableType
	columns      api.Columns
	handle       *AppenderHandle

	buffer           []*AppendRecord
	bufferLock       sync.Mutex
	bufferTicker     *time.Ticker
	bufferTickerDone sync.WaitGroup

	bufferThreshold int
}

var _ api.Appender = (*Appender)(nil)

// Close releases all resources that allocated to the Appender
func (appender *Appender) Close() (int64, int64, error) {
	if appender.appendClient == nil {
		return 0, 0, nil
	}

	if appender.bufferTicker != nil {
		appender.bufferTicker.Stop()
		appender.bufferTickerDone.Wait()
	}
	appender.flush(nil)

	client := appender.appendClient
	appender.appendClient = nil

	done, err := client.CloseAndRecv()
	if done != nil {
		return done.SuccessCount, done.FailCount, err
	} else {
		return 0, 0, err
	}
}

func (appender *Appender) TableName() string {
	return appender.tableName
}

func (appender *Appender) TableType() api.TableType {
	return appender.tableType
}

func (appender *Appender) Columns() (api.Columns, error) {
	return appender.columns, nil
}

func (appender *Appender) AppendLogTime(ts time.Time, cols ...any) error {
	if appender.tableType != api.TableTypeLog {
		return fmt.Errorf("%s is not a log table, use Append() instead", appender.tableName)
	}
	colsWithTime := append([]any{ts}, cols...)
	return appender.Append(colsWithTime...)
}

// Append appends a new record of the table.
func (appender *Appender) Append(cols ...any) error {
	if appender.appendClient == nil {
		return sql.ErrTxDone
	}

	params, err := ConvertAnyToPbTuple(cols)
	if err != nil {
		return err
	}
	err = appender.flush(&AppendRecord{Tuple: params})
	return err
}

// force flush if rec is nil
// allow buffering if rec is not nil
func (appender *Appender) flush(rec *AppendRecord) error {
	appender.bufferLock.Lock()
	defer appender.bufferLock.Unlock()

	if rec != nil {
		appender.buffer = append(appender.buffer, rec)
	}
	if len(appender.buffer) == 0 {
		return nil
	}

	if rec != nil && len(appender.buffer) < appender.bufferThreshold {
		// write new record, but not enough to flush to network
		return nil
	}

	err := appender.appendClient.Send(&AppendData{
		Handle:  appender.handle,
		Records: appender.buffer,
	})
	if err == nil {
		appender.buffer = appender.buffer[:0]
	}
	return err
}
