package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
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

func NewWithDataSource(driverName string, dataSource string) (api.Database, []api.ConnectOption, error) {
	var db *sql.DB
	var opts []api.ConnectOption
	var err error

	switch driverName {
	case "sqlite":
		db, err = sqlite.Connect(dataSource)
		break
	case "mssql":
		db, err = mssql.Connect(dataSource)
		break
	case "postgres", "postgresql":
		db, err = postgres.Connect(dataSource)
		break
	case "mysql":
		db, err = mysql.Connect(dataSource)
		break
	case "machbase":
		var host string
		var port int
		var user string = "sys"
		var password string = "manager"
		var maxOpenConn = -1
		var maxOpenQuery = -1
		var conType = 1
		if strings.Contains(dataSource, "SERVER=") {
			// input := `SERVER=value1;UID=value2;PWD=value3;CONNTYPE=1;PORT_NO=1234`
			inputSegs := strings.Split(dataSource, ";")
			for _, seg := range inputSegs {
				result := util.ParseNameValuePairs(seg)
				for _, pair := range result {
					switch strings.ToLower(pair.Name) {
					case "server":
						host = pair.Value
					case "uid":
						user = pair.Value
					case "pwd":
						password = pair.Value
					case "port_no":
						if p, err := strconv.Atoi(pair.Value); err == nil {
							port = p
						}
					case "conntype":
						if p, err := strconv.Atoi(pair.Value); err == nil {
							conType = p
						}
					}
				}
			}
		} else {
			pairs := util.ParseNameValuePairs(dataSource)
			for _, pair := range pairs {
				switch strings.ToLower(pair.Name) {
				case "host":
					host = pair.Value
				case "port":
					if p, err := strconv.Atoi(pair.Value); err == nil {
						port = p
					}
				case "user":
					user = pair.Value
				case "password":
					password = pair.Value
				case "maxopenconn":
					if p, err := strconv.Atoi(pair.Value); err == nil {
						maxOpenConn = p
					}
				case "maxopenquery":
					if p, err := strconv.Atoi(pair.Value); err == nil {
						maxOpenQuery = p
					}
				}
			}
		}
		if user != "" {
			opts = append(opts, api.WithPassword(user, password))
		}
		db, err := machcli.NewDatabase(&machcli.Config{
			Host:         host,
			Port:         port,
			TrustUsers:   map[string]string{},
			MaxOpenConn:  maxOpenConn,
			MaxOpenQuery: maxOpenQuery,
			ConType:      conType,
		})
		if err != nil {
			return nil, nil, err
		}
		return db, opts, nil
	default:
		return nil, nil, fmt.Errorf("unknown database type: %s", driverName)
	}
	if err != nil {
		return nil, nil, err
	}
	ret := &BridgedDatabase{db: db, dbType: driverName, dbConnect: dataSource}
	return ret, opts, nil
}

func SetDatabase(name string, db *sql.DB, dbType string, dbConn string) {
	if db == nil {
		panic("db is nil")
	}
	databasesLock.Lock()
	defer databasesLock.Unlock()
	databases[name] = &BridgedDatabase{db: db, dbType: dbType, dbConnect: dbConn}
}
