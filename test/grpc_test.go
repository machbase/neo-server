package test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/machbase/neo-grpc/machrpc"
	"github.com/stretchr/testify/require"
)

func TestGrpc(t *testing.T) {
	const dropTable = true
	var tableExists bool
	var count int
	var tableName = strings.ToUpper("tagdata")

	client := machrpc.NewClient(machrpc.QueryTimeout(3 * time.Second))
	err := client.Connect("unix://../tmp/mach.sock")
	//err := client.Connect("tcp://127.0.0.1:4056")
	require.Nil(t, err)
	defer client.Disconnect()

	row := client.QueryRow("select count(*) from M$SYS_TABLES where name = ?", tableName)
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
			err = client.Exec("drop table " + tableName)
			if err != nil {
				t.Logf("drop table: %s", err.Error())
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

		err := client.Exec(sqlText)
		if err != nil {
			t.Log(err.Error())
		}
		require.Nil(t, err)

		err = client.Exec(fmt.Sprintf("CREATE INDEX %s_id_idx ON %s (id)", tableName, tableName))
		if err != nil {
			t.Log(err.Error())
		}
		require.Nil(t, err)
	}

	idgen := uuid.NewGen()

	////////////
	// QueryRow
	row = client.QueryRow("select count(*) from " + tableName)
	err = row.Scan(&count)
	if err != nil {
		t.Log(err.Error())
	}
	require.Nil(t, err)
	t.Logf("count = %d", count)

	id, _ := idgen.NewV6()
	client.Exec("insert into "+tableName+" (name, time, value, id) values(?, ?, ?, ?)",
		fmt.Sprintf("name-%02d", count+1),
		time.Now(),
		0.1001+0.1001*float32(count),
		id.String())

	////////////
	// Append
	appender, err := client.Appender(tableName)
	if err != nil {
		t.Log(err.Error())
	}
	require.Nil(t, err)
	for i := 0; i < 10; i++ {
		id, _ := idgen.NewV6()
		err := appender.Append(
			fmt.Sprintf("name-%2d", count+2+i), time.Now(), 0.1001+0.1001*float32(count+1+i), "float64", nil, nil, id.String(),
			"pname", 0, nil)
		require.Nil(t, err)
	}
	appender.Close()

	////////////
	// Query
	rows, err := client.Query("select name, time, value, id from "+tableName+" where id > ?", "")
	require.Nil(t, err)
	defer rows.Close()
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

	////////////
	// QueryRow
	row = client.QueryRow("select count(*) from tagdata where id > ?", "")
	if row.Err() != nil {
		fmt.Printf("ERR> %s\n", row.Err().Error())
	}
	err = row.Scan(&count)
	require.Nil(t, err)
	t.Logf("count = %d", count)
	require.Nil(t, err)
}
