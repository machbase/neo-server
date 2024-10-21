package machrpc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"
)

func init() {
	sql.Register(Name, &NeoDriver{})
}

const Name = "machbase"

var configRegistry = map[string]*DataSource{}

func RegisterDataSource(name string, conf *DataSource) {
	configRegistry[name] = conf
}

type DataSource struct {
	ServerAddr string
	ServerCert string
	ClientKey  string
	ClientCert string
	User       string
	Password   string
}

func (conf *DataSource) newClient() (*Client, error) {
	return NewClient(&Config{
		ServerAddr: conf.ServerAddr,
		Tls: &TlsConfig{
			ClientCert: conf.ClientCert,
			ClientKey:  conf.ClientKey,
			ServerCert: conf.ServerCert,
		},
	})
}

type NeoDriver struct {
}

var _ driver.Driver = &NeoDriver{}
var _ driver.DriverContext = &NeoDriver{}

func parseDataSourceName(name string) (*DataSource, error) {
	u, err := url.Parse(name)
	if err != nil {
		return nil, err
	}
	addr := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
	var user, password string
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	}
	vals := u.Query()
	var serverCert string
	if serverCerts, ok := vals["server-cert"]; ok && len(serverCerts) > 0 {
		serverCert = serverCerts[0]
	}
	return &DataSource{
		ServerAddr: addr,
		ServerCert: serverCert,
		User:       user,
		Password:   password,
	}, nil
}

func makeClientConfig(dsn string) (*DataSource, error) {
	var conf *DataSource
	if c, ok := configRegistry[dsn]; ok {
		conf = c
	} else {
		parsedConf, err := parseDataSourceName(dsn)
		if err != nil {
			return nil, err
		}
		conf = parsedConf
	}
	return conf, nil
}

// implements sql.Driver
func (d *NeoDriver) Open(name string) (driver.Conn, error) {
	ds, err := makeClientConfig(name)
	if err != nil {
		return nil, err
	}
	client, err := ds.newClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, WithPassword(ds.User, ds.Password))
	if err != nil {
		return nil, err
	}

	ret := &NeoConn{
		name: name,
		conn: conn,
	}
	return ret, nil
}

// implements sql.DriverContext
func (d *NeoDriver) OpenConnector(name string) (driver.Connector, error) {
	ds, err := makeClientConfig(name)
	if err != nil {
		return nil, err
	}
	client, err := ds.newClient()
	if err != nil {
		return nil, err
	}

	ok, err := client.UserAuth(ds.User, ds.Password)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("invalid username or password")
	}
	conn := &NeoConnector{
		name:     name,
		driver:   d,
		client:   client,
		user:     ds.User,
		password: ds.Password,
	}
	return conn, nil
}

type NeoConnector struct {
	driver.Connector
	name     string
	driver   *NeoDriver
	client   *Client
	user     string
	password string
}

func (cn *NeoConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := cn.client.Connect(ctx, WithPassword(cn.user, cn.password))
	if err != nil {
		return nil, err
	}
	ret := &NeoConn{
		name: cn.name,
		conn: conn,
	}
	return ret, nil
}

func (cn *NeoConnector) Driver() driver.Driver {
	return cn.driver
}

type NeoConn struct {
	driver.Conn
	driver.Pinger
	driver.ConnBeginTx
	driver.QueryerContext
	driver.ExecerContext
	driver.ConnPrepareContext

	name string
	conn *Conn
}

func (c *NeoConn) Close() error {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	return nil
}

func (c *NeoConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *NeoConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return &NeoTx{}, nil
}

func (c *NeoConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]any, len(args))
	for i := range args {
		vals[i] = args[i].Value
	}
	rows, err := c.conn.Query(ctx, query, vals...)
	if err != nil {
		return nil, err
	}
	return &NeoRows{rows: rows}, nil
}

func (c *NeoConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]any, len(args))
	for i := range args {
		vals[i] = args[i].Value
	}
	row := c.conn.QueryRow(ctx, query, vals...)
	if row.Err() != nil {
		return nil, row.Err()
	}
	return &NeoResult{row: row}, nil
}

func (c *NeoConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *NeoConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	stmt := &NeoStmt{
		ctx:     ctx,
		conn:    c.conn,
		sqlText: query,
	}
	return stmt, nil
}

func (c *NeoConn) Ping(ctx context.Context) error {
	// return driver.ErrBadConn
	return nil
}

type NeoTx struct {
}

func (tx *NeoTx) Commit() error {
	return nil
}

func (tx *NeoTx) Rollback() error {
	return errors.New("Rollback method is not supported")
}

type NeoStmt struct {
	driver.Stmt
	driver.StmtExecContext
	driver.StmtQueryContext

	ctx     context.Context
	conn    *Conn
	sqlText string
}

func (stmt *NeoStmt) Close() error {
	return nil
}

func (stmt *NeoStmt) NumInput() int {
	return -1
}

func (stmt *NeoStmt) Exec(args []driver.Value) (driver.Result, error) {
	vals := make([]any, len(args))
	for i := range args {
		vals[i] = args[i]
	}
	row := stmt.conn.QueryRow(context.TODO(), stmt.sqlText, vals...)
	if row.Err() != nil {
		return nil, row.Err()
	}
	return &NeoResult{row: row}, nil
}

func (stmt *NeoStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]any, len(args))
	for i := range args {
		vals[i] = args[i].Value
	}
	row := stmt.conn.QueryRow(ctx, stmt.sqlText, vals...)
	if row.Err() != nil {
		return nil, row.Err()
	}
	return &NeoResult{row: row}, nil
}

func (stmt *NeoStmt) Query(args []driver.Value) (driver.Rows, error) {
	var ctx context.Context
	if stmt.ctx != nil {
		ctx = stmt.ctx
	} else {
		ctx = context.TODO()
	}
	vals := make([]any, len(args))
	for i := range args {
		vals[i] = args[i]
	}
	rows, err := stmt.conn.Query(ctx, stmt.sqlText, vals...)
	if err != nil {
		return nil, err
	}
	return &NeoRows{rows: rows}, nil
}

func (stmt *NeoStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]any, len(args))
	for i := range args {
		vals[i] = args[i]
	}
	rows, err := stmt.conn.Query(ctx, stmt.sqlText, vals...)
	if err != nil {
		return nil, err
	}
	return &NeoRows{rows: rows}, nil
}

type NeoResult struct {
	row *Row
}

func (r *NeoResult) LastInsertId() (int64, error) {
	return 0, errors.New("LastInsertId is not implemented")
}

func (r *NeoResult) RowsAffected() (int64, error) {
	if r.row == nil {
		return 0, nil
	}
	return r.row.RowsAffected(), nil
}

type NeoRows struct {
	rows     *Rows
	colNames []string
}

func (r *NeoRows) Columns() []string {
	if r.colNames == nil {
		r.colNames, _, _ = r.rows.Columns()
	}
	return r.colNames
}

func (r *NeoRows) Close() error {
	if r.rows == nil {
		return nil
	}
	err := r.rows.Close()
	if err != nil {
		return err
	}
	r.rows = nil
	return nil
}

func (r *NeoRows) Next(dest []driver.Value) error {
	if !r.rows.Next() {
		return io.EOF
	}
	vals := make([]any, len(dest))
	for i := range dest {
		vals[i] = &dest[i]
	}
	return r.rows.Scan(vals...)
}
