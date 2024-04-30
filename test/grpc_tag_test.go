package test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-client/machrpc"
	"github.com/stretchr/testify/require"
)

func TestGrpcTagTable(t *testing.T) {
	const dropTable = true
	var tableExists bool
	var count int
	var tableName = strings.ToUpper("tagdata")

	client, err := machrpc.NewClient(&machrpc.Config{
		ServerAddr:   "unix://../tmp/mach.sock",
		QueryTimeout: 10 * time.Second,
		Tls: &machrpc.TlsConfig{
			ClientKey:  "../tmp/machbase_pref/cert/machbase_key.pem",
			ClientCert: "../tmp/machbase_pref/cert/machbase_cert.pem",
			ServerCert: "../tmp/machbase_pref/cert/machbase_cert.pem",
		},
	})

	require.Nil(t, err)
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := client.Connect(ctx, machrpc.WithPassword("sys", "manager"))
	if err != nil {
		t.Error(err.Error())
	}
	defer conn.Close()

	row := conn.QueryRow(ctx, "select count(*) from M$SYS_TABLES where name = ?", tableName)
	require.NotNil(t, row)
	if row.Err() != nil {
		panic(row.Err())
	}
	require.Nil(t, row.Err())

	err = row.Scan(&count)
	if err == nil && count == 1 {
		tableExists = true
		t.Logf("table '%s' exists", tableName)
		if dropTable {
			t.Logf("drop table '%s'", tableName)
			result := conn.Exec(ctx, "drop table "+tableName)
			if result.Err() != nil {
				t.Logf("drop table: %s", result.Err().Error())
			}
			require.Nil(t, err)
			tableExists = false
		}
	}

	////////////
	// Exec
	if !tableExists {
		t.Logf("table '%s' doesn't exist, create new one", tableName)

		sqlText := fmt.Sprintf(`
			create tag table %s ( 
				name            varchar(200) primary key, 
				time            datetime basetime, 
				value           double summarized, 
				type            varchar(40),
				ivalue          long,
				svalue          varchar(400),
				id              varchar(80), 
				pname           varchar(80),
				sampling_period long,
				payload         json
			)`, tableName)

		result := conn.Exec(ctx, sqlText)
		if result.Err() != nil {
			panic(result.Err())
		}
		require.Nil(t, err)

		//TODO remove comment when tag index is ready, MACH-ERR 2334 Tag Index is not yet supported.
		// result = client.Exec(fmt.Sprintf("CREATE INDEX %s_id_idx ON %s (id)", tableName, tableName))
		// if result.Err() != nil {
		// 	panic(result.Err())
		// }
		// require.Nil(t, err)
	}

	idgen := uuid.NewGen()

	////////////
	// QueryRow
	row = conn.QueryRow(ctx, "select count(*) from "+tableName)
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}
	require.Nil(t, err)
	t.Logf("count = %d", count)

	id, _ := idgen.NewV6()
	result := conn.Exec(ctx, "insert into "+tableName+" (name, time, value, id) values(?, ?, ?, ?)",
		fmt.Sprintf("name-%02d", count+1),
		time.Now(),
		0.1001+0.1001*float32(count),
		id.String())
	if result.Err() != nil {
		panic(result.Err())
	}
	require.Nil(t, err)
	count++

	////////////
	// Append - tag table
	appender, err := conn.Appender(ctx, tableName)
	if err != nil {
		t.Log(err.Error())
	}
	require.Nil(t, err)

	for i := 0; i < 10; i++ {
		id, _ := idgen.NewV6()
		err := appender.Append(
			fmt.Sprintf("name-%d", count%5),
			time.Now(),
			0.1001+0.1001*float64(count+1+i),
			"float64",
			nil,
			nil,
			id.String(),
			"pname",
			0,
			nil)
		require.Nil(t, err)
	}
	appender.Close()

	////////////
	// Query
	rows, err := conn.Query(ctx, "select name, time, value, id from "+tableName)
	require.Nil(t, err)
	//defer rows.Close()
	for rows.Next() {
		var name string
		var ts time.Time
		var value float64
		var id string
		err := rows.Scan(&name, &ts, &value, &id)
		if err != nil {
			t.Logf("ERR> %v", err.Error())
			break
		}
		t.Logf("==> %v %v %v %v", name, ts, value, id)
	}
	rows.Close()

	////////////
	// QueryRow
	row = conn.QueryRow(ctx, "select count(*) from tagdata")
	if row.Err() != nil {
		fmt.Printf("ERR> %s\n", row.Err().Error())
	}
	err = row.Scan(&count)
	require.Nil(t, err)
	t.Logf("count = %d", count)
	require.Nil(t, err)
}
