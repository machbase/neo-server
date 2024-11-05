package machrpc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/machbase/neo-server/api"
	"google.golang.org/grpc"
)

const DefaultDriverName = "machbase-neo"

func init() {
	sql.Register(DefaultDriverName, &Driver{})
}

var _ driver.Driver = (*Driver)(nil)
var _ driver.DriverContext = (*Driver)(nil)

type Driver struct {
	ConnProvider func() (*grpc.ClientConn, error)
	ServerAddr   string
	ServerCert   string
	ClientKey    string
	ClientCert   string
	User         string
	Password     string
}

// implements sql.Driver
func (drv *Driver) Open(dsn string) (driver.Conn, error) {
	user := drv.User
	password := drv.Password
	var client *Client
	var err error
	if drv.ConnProvider != nil {
		conf := &Config{
			ConnProvider: drv.ConnProvider,
		}
		parseConnectionString(conf, dsn, &user, &password)
		client, err = NewClient(conf)
	} else {
		conf := &Config{
			ServerAddr: drv.ServerAddr,
			Tls: &TlsConfig{
				ClientCert: drv.ClientCert,
				ClientKey:  drv.ClientKey,
				ServerCert: drv.ServerCert,
			},
		}
		parseConnectionString(conf, dsn, &user, &password)
		client, err = NewClient(conf)
	}
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, api.WithPassword(user, password))
	if err != nil {
		return nil, err
	}

	ret := &NeoConn{
		conn: conn.(*Conn),
	}
	return ret, nil
}

// implements sql.DriverContext
func (drv *Driver) OpenConnector(dsn string) (driver.Connector, error) {
	user := drv.User
	password := drv.Password

	var client *Client
	var err error
	if drv.ConnProvider != nil {
		conf := &Config{
			ConnProvider: drv.ConnProvider,
		}
		parseConnectionString(conf, dsn, &user, &password)
		client, err = NewClient(conf)
	} else {
		conf := &Config{
			ServerAddr: drv.ServerAddr,
			Tls: &TlsConfig{
				ClientCert: drv.ClientCert,
				ClientKey:  drv.ClientKey,
				ServerCert: drv.ServerCert,
			},
		}
		parseConnectionString(conf, dsn, &user, &password)
		client, err = NewClient(conf)
	}
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ok, reason, err := client.UserAuth(ctx, user, password)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New(reason)
	}
	conn := &NeoConnector{
		driver:   drv,
		client:   client,
		user:     user,
		password: password,
	}
	return conn, nil
}

func parseConnectionString(conf *Config, str string, user *string, password *string) error {
	lines := strings.Split(str, ";")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		toks := strings.SplitN(line, "=", 2)
		if len(toks) != 2 {
			return fmt.Errorf("invalid connection string %q", line)
		}
		switch strings.ToLower(toks[0]) {
		case "server":
			u, err := url.Parse(toks[1])
			if err != nil {
				return fmt.Errorf("invalid connection server %q", toks[1])
			}
			conf.ServerAddr = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
			if u.User != nil {
				*user = u.User.Username()
				*password, _ = u.User.Password()
			}
		case "user":
			*user = toks[1]
		case "password":
			*password = toks[1]
		case "server-cert":
			if conf.Tls == nil {
				conf.Tls = &TlsConfig{}
			}
			conf.Tls.ServerCert = toks[1]
		case "client-key":
			if conf.Tls == nil {
				conf.Tls = &TlsConfig{}
			}
			conf.Tls.ClientKey = toks[1]
		case "client-cert":
			if conf.Tls == nil {
				conf.Tls = &TlsConfig{}
			}
			conf.Tls.ClientCert = toks[1]
		}
	}
	return nil
}

type NeoConnector struct {
	driver   driver.Driver
	client   *Client
	user     string
	password string
}

var _ driver.Connector = &NeoConnector{}

func (cn *NeoConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := cn.client.Connect(ctx, api.WithPassword(cn.user, cn.password))
	if err != nil {
		return nil, err
	}
	ret := &NeoConn{
		conn: conn.(*Conn),
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
	return &NeoRows{rows: rows.(*Rows)}, nil
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
	return &NeoResult{row: row.(*Row)}, nil
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
	return &NeoResult{row: row.(*Row)}, nil
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
	return &NeoResult{row: row.(*Row)}, nil
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
	return &NeoRows{rows: rows.(*Rows)}, nil
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
	return &NeoRows{rows: rows.(*Rows)}, nil
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
		if cols, err := r.rows.Columns(); err == nil {
			r.colNames = cols.Names()
		}
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
