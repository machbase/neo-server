package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/machbase/neo-client"
	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/mssql"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/mysql"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/postgres"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/sqlite"
	"github.com/machbase/neo-server/v8/mods/util"
)

func NewSqliteBridge(name string, path string) *sqlite.Bridge {
	return sqlite.NewBridge(name, path)
}

func NewPostgresBridge(name string, path string) *postgres.Bridge {
	return postgres.NewBridge(name, path)
}

func NewMySQLBridge(name string, path string) *mysql.Bridge {
	return mysql.NewBridge(name, path)
}

func NewMSSQLBridge(name string, path string) *mssql.Bridge {
	return mssql.NewBridge(name, path)
}

type BridgedDatabase struct {
	db        *sql.DB
	dbType    string
	dbConnect string
}

func (d *BridgedDatabase) Connect(ctx context.Context, options ...api.ConnectOption) (*sql.Conn, error) {
	if c, err := d.db.Conn(ctx); err != nil {
		return nil, err
	} else {
		return c, nil
	}
}

func (d *BridgedDatabase) UserAuth(ctx context.Context, user string, password string) (bool, string, error) {
	return true, "", nil
}

func (d *BridgedDatabase) Ping(ctx context.Context) (time.Duration, error) {
	tick := time.Now()
	if err := d.db.Ping(); err != nil {
		return 0, err
	}
	return time.Since(tick), nil
}

var databases = map[string]*sql.DB{}
var databasesLock sync.RWMutex

func init() {
	util.AddShutdownHook(func() {
		for _, d := range databases {
			d.Close()
		}
	})
}

func Conn(ctx context.Context, name string) (*sql.Conn, error) {
	db, err := Database(name)
	if err != nil {
		return nil, err
	}
	return db.Conn(ctx)
}

func Database(name string) (*sql.DB, error) {
	var db *sql.DB
	var dbConn string
	var dbType string
	var err error

	databasesLock.RLock()
	if dbm, ok := databases[name]; ok {
		databasesLock.RUnlock()
		return dbm, nil
	}
	databasesLock.RUnlock()

	databasesLock.Lock()
	defer databasesLock.Unlock()
	if strings.HasPrefix(name, "sqlite,") {
		dbType = "sqlite"
		dbConn = strings.TrimPrefix(name, "sqlite,")
		db, err = sqlite.Connect(dbConn)
	} else if strings.HasPrefix(name, "mssql,") {
		dbType = "mssql"
		dbConn = strings.TrimPrefix(name, "mssql,")
		db, err = mssql.Connect(dbConn)
	} else if strings.HasPrefix(name, "postgres,") {
		dbType = "postgres"
		dbConn = strings.TrimPrefix(name, "postgres,")
		db, err = postgres.Connect(dbConn)
	} else if strings.HasPrefix(name, "mysql,") {
		dbType = "mysql"
		dbConn = strings.TrimPrefix(name, "mysql,")
		db, err = mysql.Connect(dbConn)
	} else {
		return nil, fmt.Errorf("unknown database type: %s", name)
	}
	if err != nil {
		return nil, err
	}
	if db != nil {
		databases[name] = db
	}
	_ = dbType
	return db, nil
}

func New(name string) (*sql.DB, error) {
	var db *sql.DB
	var dbType string
	var dbConn string
	var err error

	databasesLock.RLock()
	if db, ok := databases[name]; ok {
		databasesLock.RUnlock()
		return db, nil
	}
	databasesLock.RUnlock()

	if strings.HasPrefix(name, "sqlite,") {
		dbType = "sqlite"
		dbConn = strings.TrimPrefix(name, "sqlite,")
		db, err = sqlite.Connect(dbConn)
	} else if strings.HasPrefix(name, "mssql,") {
		dbType = "mssql"
		dbConn = strings.TrimPrefix(name, "mssql,")
		db, err = mssql.Connect(dbConn)
	} else if strings.HasPrefix(name, "postgres,") {
		dbType = "postgres"
		dbConn = strings.TrimPrefix(name, "postgres,")
		db, err = postgres.Connect(dbConn)
	} else if strings.HasPrefix(name, "mysql,") {
		dbType = "mysql"
		dbConn = strings.TrimPrefix(name, "mysql,")
		db, err = mysql.Connect(dbConn)
	}
	if err != nil {
		return nil, err
	}
	_ = dbType
	if db != nil {
		databasesLock.Lock()
		databases[name] = db
		databasesLock.Unlock()
		return db, nil
	}

	return nil, fmt.Errorf("unknown database type: %s", name)
}

func NewWithDataSource(driverName string, dataSource string) (*sql.DB, error) {
	var db *sql.DB
	var err error

	switch driverName {
	case "sqlite":
		db, err = sqlite.Connect(dataSource)
	case "mssql":
		db, err = mssql.Connect(dataSource)
	case "postgres", "postgresql":
		db, err = postgres.Connect(dataSource)
	case "mysql":
		db, err = mysql.Connect(dataSource)
	case "machbase":
		db, err = sql.Open("machbase", dataSource)
	default:
		return nil, fmt.Errorf("unknown database type: %s", driverName)
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}

func SetDatabase(name string, db *sql.DB, dbType string, dbConn string) {
	if db == nil {
		panic("db is nil")
	}
	databasesLock.Lock()
	defer databasesLock.Unlock()
	databases[name] = db
}

func UnsetDatabase(name string) {
	databasesLock.Lock()
	defer databasesLock.Unlock()
	delete(databases, name)
}
