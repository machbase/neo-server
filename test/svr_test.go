package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/machbase/booter"
	_ "github.com/machbase/cemlib/logging"
	mach "github.com/machbase/neo-engine"
	_ "github.com/machbase/neo-server/mods/server"
)

var serverConf = []byte(`
define VARS {
	WORKDIR = "../tmp"
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
        MachbaseHome     = "${VARS_WORKDIR}/machbase"
		MachbasePreset   = 2
        Machbase = {
            HANDLE_LIMIT = 4096
			TAG_CACHE_MAX_MEMORY_SIZE = 536870912
			DISK_BUFFER_COUNT = 1
			TAG_CACHE_ENABLE = 3
        }
        Grpc = {
            Listeners        = [ 
                "unix://${VARS_WORKDIR}/mach.sock", 
                "tcp://127.0.0.1:5655",
            ]
            MaxRecvMsgSize   = 4
            MaxSendMsgSize   = 4
        }
        Http = {
            Listeners        = [ "tcp://127.0.0.1:5654" ]
            Handlers         = [
                { Prefix: "/db",       Handler: "machbase" },
                { Prefix: "/metrics",  Handler: "influx" },
				{ Prefix: "/logvault", Handler: "logvault"},
            ]
        }
        Mqtt = {
            Listeners        = [ "tcp://127.0.0.1:5653"]
            Handlers         = [
                { Prefix: "db",      Handler: "machbase" },
                { Prefix: "metrics", Handler: "influx" },
            ]
        }
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
	db := mach.New()
	var count int

	checkTableSql := fmt.Sprintf("select count(*) from M$SYS_TABLES where name = '%s'", benchmarkTableName)
	row := db.QueryRow(checkTableSql)
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}

	if count == 1 {
		dropTableSql := fmt.Sprintf("drop table %s", benchmarkTableName)
		_, err = db.Exec(dropTableSql)
		if err != nil {
			panic(err)
		}
	}

	creTableSql := fmt.Sprintf(db.SqlTidy(`
            create tag table %s (
                name     varchar(200) primary key,
                time     datetime basetime,
                value    double summarized,
                id       varchar(80),
                jsondata json
        )`), benchmarkTableName)
	_, err = db.Exec(creTableSql)
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(fmt.Sprintf("CREATE INDEX %s_id_idx ON %s (id)", benchmarkTableName, benchmarkTableName))
	if err != nil {
		panic(err)
	}

	row = db.QueryRow("select count(*) from " + benchmarkTableName)
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}
	/// end of preparing benchmark table

	m.Run()
	b.Shutdown()
}
