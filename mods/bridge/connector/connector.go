package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/mssql"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/mysql"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/postgres"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/sqlite"
	"github.com/machbase/neo-server/v8/mods/util"
)

type BridgedDatabase struct {
	db        *sql.DB
	dbType    string
	dbConnect string
}

func (d *BridgedDatabase) Connect(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
	if c, err := d.db.Conn(ctx); err != nil {
		return nil, err
	} else {
		return api.WrapSqlConn(c), nil
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

var databases = map[string]*BridgedDatabase{}
var databasesLock sync.RWMutex

func init() {
	util.AddShutdownHook(func() {
		for _, d := range databases {
			d.db.Close()
		}
	})
}

func New(name string) (api.Database, error) {
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
	if db != nil {
		ret := &BridgedDatabase{db: db, dbType: dbType, dbConnect: dbConn}
		databasesLock.Lock()
		databases[name] = ret
		databasesLock.Unlock()
		return ret, nil
	}

	return nil, fmt.Errorf("unknown database type: %s", name)
}

func NewWithDataSource(driverName string, dataSourceName string) (api.Database, error) {
	var db *sql.DB
	var err error

	switch driverName {
	case "sqlite":
		db, err = sqlite.Connect(dataSourceName)
		break
	case "mssql":
		db, err = mssql.Connect(dataSourceName)
		break
	case "postgres":
		db, err = postgres.Connect(dataSourceName)
		break
	case "mysql":
		db, err = mysql.Connect(dataSourceName)
		break
	default:
		return nil, fmt.Errorf("unknown database type: %s", driverName)
	}
	if err != nil {
		return nil, err
	}
	ret := &BridgedDatabase{db: db, dbType: driverName, dbConnect: dataSourceName}
	return ret, nil
}

func SetDatabase(name string, db *sql.DB, dbType string, dbConn string) {
	if db == nil {
		panic("db is nil")
	}
	databasesLock.Lock()
	defer databasesLock.Unlock()
	databases[name] = &BridgedDatabase{db: db, dbType: dbType, dbConnect: dbConn}
}
