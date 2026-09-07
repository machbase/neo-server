package machcli

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/spi"
)

//go:embed machcli.js
var machcli_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"machcli.js": machcli_js,
	}
}

func Module(ctx context.Context, rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	exports := module.Get("exports").(*goja.Object)
	exports.Set("NewDatabase", func(data string) (*Database, error) {
		return newDatabase(ctx, data)
	})
	exports.Set("Context", Context)
	exports.Set("Unbox", Unbox)
	exports.Set("RowsScan", RowsScan)
	exports.Set("QueryRow", QueryRow)
	exports.Set("Explain", Explain)
	exports.Set("Message", Message)
	exports.Set("IsFetchable", IsFetchable)
	exports.Set("BeginTx", BeginTx)
	exports.Set("Named", func(name string, value any) sql.NamedArg {
		return sql.Named(name, value)
	})
}

func Unbox(value any) any {
	v := client.Unbox(value)
	switch v := v.(type) {
	case api.JSONString:
		return string(v)
	}
	return v
}

func Context(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, client.MetaKey, &client.Meta{})
	return ctx
}

type Config struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	Password        string `json:"password"`
	IdentityFile    string `json:"identityFile,omitempty"`
	AlternativeHost string `json:"alternativeHost,omitempty"`
	AlternativePort int    `json:"alternativePort,omitempty"`
	Database        string `json:"database,omitempty"`
	DB              string `json:"db,omitempty"` // alias of Database
}

type Database struct {
	Ctx  context.Context
	pool *sql.DB
	user string
	dsn  string
}

func NewDatabase(data string) (*Database, error) {
	return newDatabase(context.Background(), data)
}

func newDatabase(ctx context.Context, data string) (*Database, error) {
	obj := Config{
		Host:     "127.0.0.1",
		Port:     5656,
		User:     "",
		Password: "",
	} // default values
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, err
	}
	if obj.Database == "" {
		obj.Database = obj.DB
	}
	opts := []string{
		"host=" + obj.Host,
		"port=" + fmt.Sprintf("%d", obj.Port),
		"user=" + obj.User,
	}
	if obj.AlternativeHost != "" && obj.AlternativePort != 0 {
		opts = append(opts, fmt.Sprintf("alternative_servers=%s:%d", obj.AlternativeHost, obj.AlternativePort))
	}
	if obj.Database != "" {
		opts = append(opts, "database="+obj.Database)
	}
	if obj.Password != "" {
		opts = append(opts, "password="+obj.Password)
	} else {
		if strings.HasPrefix(obj.IdentityFile, "@") {
			// path is os filesystem
			opts = append(opts, "auth_key_file="+strings.TrimPrefix(obj.IdentityFile, "@"))
		} else {
			// path is virtual filesystem
			// TODO: load private key from virtual filesystem
			return nil, fmt.Errorf("loading private key from virtual filesystem is not supported yet")
		}
	}
	dsn := strings.Join(opts, ";")
	db, err := sql.Open("machbase", dsn)
	if err != nil {
		return nil, err
	}
	return &Database{
		Ctx:  ctx,
		pool: db,
		user: strings.ToUpper(obj.User),
		dsn:  dsn,
	}, nil
}

func (db *Database) Close() error {
	if db.pool == nil {
		return nil
	}
	return db.pool.Close()
}

func (db *Database) User() string {
	return db.user
}

func (db *Database) Connect() (*sql.Conn, error) {
	return db.pool.Conn(db.Ctx)
}

// BeginTx starts a transaction on target, which must be either a *Database
// (a pooled connection is acquired implicitly, mirroring client.Tx) or a
// *sql.Conn (the transaction runs on that specific connection, mirroring
// client.TxConn). machbase does not support transaction options, so the
// default options are always used.
func BeginTx(ctx context.Context, target any) (*sql.Tx, error) {
	switch t := target.(type) {
	case *Database:
		return t.pool.BeginTx(ctx, nil)
	case *sql.Conn:
		return t.BeginTx(ctx, nil)
	default:
		return nil, fmt.Errorf("unsupported target for BeginTx: %T", target)
	}
}

func (db *Database) Appender(ctx context.Context, table string, columns ...string) (*client.Appender, error) {
	ret := &client.Appender{}
	if err := ret.Connect(ctx, db.dsn, table, columns...); err != nil {
		return nil, err
	}
	return ret, nil
}

func (db *Database) NormalizeTableName(tableName string) [3]string {
	tableName = strings.ToUpper(tableName)
	toks := strings.Split(tableName, ".")
	if len(toks) == 1 {
		return [3]string{"MACHBASEDB", db.user, toks[0]}
	} else if len(toks) == 2 {
		return [3]string{"MACHBASEDB", toks[0], toks[1]}
	} else if len(toks) == 3 {
		return [3]string{toks[0], toks[1], toks[2]}
	}
	return [3]string{"", "", tableName}
}

// queryContexter is satisfied by both *sql.Conn and *sql.Tx, so QueryRow works
// the same way inside a Tx() closure as it does on a plain connection.
type queryContexter interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func QueryRow(ctx context.Context, conn any, sqlText string, args ...any) (map[string]any, error) {
	q, ok := conn.(queryContexter)
	if !ok {
		return nil, fmt.Errorf("unsupported query target: %T", conn)
	}
	rows, err := q.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	cols, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	buffer := spi.MakeBuffer(cols)
	if err := rows.Scan(buffer...); err != nil {
		return nil, err
	}
	result := make(map[string]any)
	result["_ROWNUM"] = 1
	for i, col := range cols {
		result[col.Name()] = Unbox(buffer[i])
	}
	return result, nil
}

// This helper function is used to fetch rows that includes null values,
// which are not properly-handled by goja's variadic arguments in rows.Scan(...buffer).
func RowsScan(rows *sql.Rows) ([]any, error) {
	cols, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	buffer := spi.MakeBuffer(cols)
	err = rows.Scan(buffer...)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

type Explainer interface {
	Explain(ctx context.Context, sqlText string, full bool) (string, error)
}

func Explain(ctx context.Context, conn any, sqlText string, args ...any) (plan string, err error) {
	dbConn, ok := conn.(*sql.Conn)
	if !ok {
		return "", fmt.Errorf("explain is not supported on %T", conn)
	}
	dbConn.Raw(func(driverConn any) error {
		if c, ok := driverConn.(Explainer); ok {
			full := false
			if len(args) > 0 {
				if b, ok := args[0].(bool); ok {
					full = b
				}
			}
			plan, err = c.Explain(ctx, sqlText, full)
		} else {
			err = fmt.Errorf("driver does not support Explain interface")
		}
		return nil
	})
	return
}

func IsFetchable(ctx context.Context) bool {
	meta := ctx.Value(client.MetaKey)
	if meta == nil {
		return false
	}
	if m, ok := meta.(*client.Meta); ok {
		return m.IsFetchable()
	}
	return false
}

func Message(ctx context.Context) string {
	meta := ctx.Value(client.MetaKey)
	if meta == nil {
		return ""
	}
	if m, ok := meta.(*client.Meta); ok {
		return m.Message()
	}
	return ""
}
