package test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/booter"
	_ "github.com/machbase/neo-server/v8/mods/logging"
	_ "github.com/machbase/neo-server/v8/mods/server"
)

var serverConf = []byte(`
define VARS {
	WORKDIR = "./tmp"
}

module "machbase.com/neo-logging" {
    config {
        Console                     = false
        Filename                    = "-"
        DefaultPrefixWidth          = 30
        DefaultEnableSourceLocation = true
        DefaultLevel                = "TRACE"
        Levels = [
            { Pattern="machsvr", Level="TRACE" },
        ]
    }
}

module "machbase.com/neo-server" {
    name = "machsvr"
    config {
		PrefDir  = "${VARS_WORKDIR}/machbase_pref"
        DataDir  = "${VARS_WORKDIR}/machbase_home"
        Machbase = {
			PORT_NO = 5566
            HANDLE_LIMIT = 4096
			TAG_CACHE_MAX_MEMORY_SIZE = 536870912
			DISK_BUFFER_COUNT = 1
			TAG_CACHE_ENABLE = 3
        }
        Grpc = {
            Listeners        = [ 
                "unix://${VARS_WORKDIR}/mach.sock", 
                "tcp://127.0.0.1:6565",
            ]
            MaxRecvMsgSize   = 4
            MaxSendMsgSize   = 4
        }
        Http = {
            Listeners        = [ "tcp://127.0.0.1:5654" ]
        }
        Mqtt = {
            Listeners        = [ "tcp://127.0.0.1:5653"]
        }
		MachbaseInitOption       = 2
    }
}
`)

var lineProtocolData = `cpu,cpu=cpu-total,host=desktop usage_irq=0,usage_softirq=0.004171359446037821,usage_guest=0,usage_user=0.3253660367906774,usage_system=0.0792558294748905,usage_idle=99.59120677410203,usage_guest_nice=0,usage_nice=0,usage_iowait=0,usage_steal=0 1670975120000000000
mem,host=desktop committed_as=8780218368i,dirty=327680i,huge_pages_free=0i,shared=67067904i,sreclaimable=414224384i,total=67377881088i,buffered=810778624i,vmalloc_total=35184372087808i,active=3356581888i,available_percent=95.04513097460023,free=56726638592i,slab=617472000i,available=64039395328i,vmalloc_used=54685696i,cached=7298387968i,inactive=6323064832i,low_total=0i,page_tables=32129024i,high_free=0i,commit_limit=35836420096i,high_total=0i,swap_total=2147479552i,write_back_tmp=0i,write_back=0i,used=2542075904i,swap_cached=0i,vmalloc_chunk=0i,mapped=652132352i,huge_page_size=2097152i,huge_pages_total=0i,low_free=0i,sunreclaim=203247616i,swap_free=2147479552i,used_percent=3.7728641253646424 1670975120000000000
disk,device=nvme0n1p3,fstype=ext4,host=desktop,mode=rw,path=/ total=1967315451904i,free=1823398948864i,used=43906785280i,used_percent=2.3513442109214915,inodes_total=122068992i,inodes_free=121125115i,inodes_used=943877i 1670975120000000000
system,host=desktop n_users=2i,load1=0.08,load5=0.1,load15=0.09,n_cpus=24i 1670975120000000000
system,host=desktop uptime=513536i 1670975120000000000
system,host=desktop uptime_format="5 days, 22:38" 1670975120000000000
processes,host=desktop zombies=0i,unknown=0i,dead=0i,paging=0i,total_threads=1084i,blocked=0i,stopped=0i,running=0i,sleeping=282i,total=426i,idle=144i 1670975120000000000`

var benchmarkTableName = strings.ToUpper("samplebench")

func TestMain(m *testing.M) {
	os.Mkdir("./tmp", 0755)
	defer func() {
		os.RemoveAll("./tmp")
		e := recover()
		if e == nil {
			return
		}
		fmt.Println(e)
	}()

	builder := booter.NewBuilder()
	b, err := builder.BuildWithContent(serverConf)
	if err != nil {
		panic(err)
	}
	err = b.Startup()
	if err != nil {
		panic(err)
	}

	/// preparing benchmark table
	db, err := machsvr.NewDatabase(machsvr.DatabaseOption{})
	if err != nil {
		panic(err)
	}
	var count int

	api.StartAppendWorkers()

	checkTableSql := fmt.Sprintf("select count(*) from M$SYS_TABLES where name = '%s'", benchmarkTableName)
	conn, err := db.Connect(context.TODO(), api.WithTrustUser("sys"))
	if err != nil {
		panic(err)
	}
	row := conn.QueryRow(context.TODO(), checkTableSql)
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}

	if count == 1 {
		dropTableSql := fmt.Sprintf("drop table %s", benchmarkTableName)
		result := conn.Exec(context.TODO(), dropTableSql)
		if result.Err() != nil {
			panic(result.Err())
		}
	}

	creTableSql := fmt.Sprintf(SqlTidy(`
            create tag table %s (
                name     varchar(200) primary key,
                time     datetime basetime,
                value    double summarized,
                id       varchar(80),
                jsondata json
        )`), benchmarkTableName)
	result := conn.Exec(context.TODO(), creTableSql)
	if result.Err() != nil {
		panic(result.Err())
	}

	result = conn.Exec(context.TODO(), fmt.Sprintf("CREATE INDEX %s_id_idx ON %s (id)", benchmarkTableName, benchmarkTableName))
	if result.Err() != nil {
		panic(result.Err())
	}

	row = conn.QueryRow(context.TODO(), "select count(*) from "+benchmarkTableName)
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}
	/// end of preparing benchmark table

	flushTable = func(tableName string) {
		ctx := context.TODO()
		conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		conn.Exec(ctx, fmt.Sprintf("exec table_flush(%s)", tableName))
	}

	m.Run()

	// cancel Conn test
	rows, err := conn.Query(context.TODO(), "select * from "+benchmarkTableName+" order by time ")
	if err != nil {
		panic(err)
	}
	if !conn.(*machsvr.Conn).Connected() {
		panic("connection is not connected")
	}
	listNeoSession(db)
	if err := conn.(*machsvr.Conn).Cancel(); err != nil {
		panic(err)
	}
	rows.Close()
	listNeoSession(db)

	api.StopAppendWorkers()
	// shutdown
	b.Shutdown()
	time.Sleep(3 * time.Second)
}

func SqlTidy(sqlText string) string {
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

func listNeoSession(db *machsvr.Database) {
	ctx := context.TODO()
	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	// ID     │ USER_ID │ USER_NAME │ STMT_COUNT │
	rows, err := conn.Query(ctx, "select id, user_name from V$NEO_SESSION")
	if err != nil {
		panic(err)
	}
	fmt.Println("---------- V$NEO_SESSION ----------")
	for rows.Next() {
		var id, user string
		if err := rows.Scan(&id, &user); err != nil {
			panic(err)
		}
		if cw, ok := db.GetWatcher(id); ok {
			fmt.Printf("ID: %s, USER: %s %s\n", id, user, cw.LatestSql)
		} else {
			fmt.Printf("ID: %s, USER: %s -- watcher not found\n", id, user)
		}
	}
}

var flushTable func(tableName string)
