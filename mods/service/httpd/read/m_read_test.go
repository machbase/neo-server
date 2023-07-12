package httpd

import (
	"encoding/json"
	"testing"

	"github.com/machbase/neo-grpc/machrpc"
	spi "github.com/machbase/neo-spi"
)

const (
	TagTableName  = "tagdata"
	ConfTableName = "tagconf"
	NeoServeAddr  = "unix://../tmp/mach-grpc.sock"
)

type neo struct {
	cli spi.DatabaseClient
}

func TestNeo(t *testing.T) {
	neo := neo{}

	neo.cli = machrpc.NewClient(machrpc.WithServer(NeoServeAddr))
	err := neo.cli.Connect()
	if err != nil {
		t.Fatal(err)
	}

	_, err = neo.cli.GetServerInfo()
	if err != nil {
		t.Fatal(err)
	}

	sqlText := `select * from tagdata limit 10;`

	rows, err := neo.cli.Query(sqlText)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}

	colNames := cols.Names()
	colTypes := cols.Types()

	t.Logf("colNames : %+v", colNames)
	t.Logf("colTypes : %+v", colTypes)
	t.Logf("message : %+v", rows.Message())

	bufx := cols.MakeBuffer()

	var datas []interface{}
	for rows.Next() {
		err = rows.Scan(bufx...)
		if err != nil {
			t.Fatal(err)
		}
		bdata, err := json.Marshal(bufx)
		if err != nil {
			t.Fatal(err)
		}

		var data interface{}
		err = json.Unmarshal(bdata, &data)
		if err != nil {
			t.Fatal(err)
		}
		datas = append(datas, data)
	}

	t.Logf("%+v", datas)
}

// t.Logf("serverInfo: %+v", serverInfo)

// var dropTable bool = true
// if err = neo.TableCheck(dropTable); err != nil {
// 	log.Fatal("TestTableCheck error : ", err)
// }

// func (n *neo) TableCheck(dropTable bool) error {
// 	tableNames := []string{TagTableName, ConfTableName}

// 	for _, tableName := range tableNames {
// 		tableExists := false
// 		row := n.cli.QueryRow("select count(*) from M$SYS_TABLES where name = ?", strings.ToUpper(tableName))
// 		fmt.Printf("row: %+v\n", row)
// 		if row.Err() == nil {
// 			var count int
// 			err := row.Scan(&count)
// 			if err == nil && count == 1 {
// 				tableExists = true
// 				fmt.Printf("%s table exist\n", tableName)
// 				if dropTable {
// 					fmt.Printf("drop table %q\n", tableName)
// 					result := n.cli.Exec(fmt.Sprintf("drop table %s", tableName))
// 					if result.Err() == nil {
// 						tableExists = false
// 					}
// 				}
// 			}
// 		}
// 		if !tableExists {
// 			fmt.Printf("create table %q\n", tableName)
// 			if tableName == TagTableName {
// 				if err := n.createTagTable(); err != nil {
// 					return errors.Wrapf(err, "create %s table", tableName)
// 				}
// 			} else if tableName == ConfTableName {
// 				if err := n.createLookUpTable(); err != nil {
// 					return errors.Wrapf(err, "create %s table", tableName)
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

// func (n *neo) createTagTable() error {

// 	sqlText := fmt.Sprintf(`
// 		create tag table %s (
// 			name            varchar(200) primary key,
// 			time            datetime basetime,
// 			value           double summarized,
// 			type            varchar(40),
// 			ivalue          long,
// 			svalue          varchar(400),
// 			id              varchar(80),
// 			pname           varchar(80),
// 			sampling_period long,
// 			payload         json
// 		)`, TagTableName)

// 	result := n.cli.Exec(sqlText)
// 	if result.Err() != nil {
// 		return errors.Wrap(result.Err(), "create tagdata table")
// 	}

// 	sqlText = fmt.Sprintf("CREATE INDEX %s_id_idx ON %s (id)", TagTableName, TagTableName)
// 	result = n.cli.Exec(sqlText)
// 	if result.Err() != nil {
// 		return errors.Wrap(result.Err(), "create index")
// 	}

// 	return nil
// }

// func (n *neo) createLookUpTable() error {
// 	sqlText := fmt.Sprintf(`
// 		create lookup table %s (
// 			tagname VARCHAR(40) PRIMARY KEY,
// 			feature VARCHAR(10),
// 			data	VARCHAR(512)
// 		)`, ConfTableName)

// 	result := n.cli.Exec(sqlText)
// 	if result.Err() != nil {
// 		return errors.Wrap(result.Err(), "create tagconf table")
// 	}

// 	return nil
// }
