package tql_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTqlSqlExplain(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_explain",
			Script: `
				SQL('explain select * from tag_data')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.Greater(t, len(result), 50, result)
				require.Contains(t, result, "TAG READ (RAW)")
			},
		},
		{
			Name: "SQL_explain_full",
			Script: `
				SQL('explain full select * from tag_data')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.Greater(t, len(result), 5000, result)
				require.Contains(t, result, "EXECUTE")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSql(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_sink",
			Script: `
				SCRIPT({
					const dt = new Date('2026-07-10T17:10:20');
					$.yield(
						'sql_test', dt, 3.142, 			// name, time, value
						-123, 123,						// short, ushort
						-1234, 1234,					// int, uint
						-12345, 12345,					// long, ulong
						'STR', '{"json":true}',			// str, json
						'192.168.0.1', '2001:db8::1',	// ipv4, ipv6
						new Uint8Array([1,2,3]) 		// bin
				)})
				SQL('insert into tag_data (name,time,value, '+
					'short_value,ushort_value,int_value,uint_value, '+
					'long_value,ulong_value,str_value,json_value,ipv4_value,ipv6_value,bin_value) '+
					'values(?,?,?,?,?,?,?,?,?,?,?,?,?,?)',
						value(0), value(1), value(2),
						value(3), value(4), value(5), value(6),
						value(7), value(8), value(9), value(10), value(11), value(12), value(13)
				)
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, gjson.Get(result, "data.message").String(), "a row inserted.")
			},
		},
		{
			Name: "SQL_FLUSH",
			Script: `
				FAKE(once(1))
				SQL('exec table_flush(tag_data)')
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, gjson.Get(result, "data.message").String(), "executed.")
			},
		},
		{
			Name: "SQL_csv",
			Script: `
				SQL('select * from tag_data where name = ?', 'sql_test')
				CSV(header(true), timeformat('default'), tz('Local'))
			`,
			ExpectCSV: []string{
				"NAME,TIME,VALUE,SHORT_VALUE,USHORT_VALUE,INT_VALUE,UINT_VALUE,LONG_VALUE,ULONG_VALUE,STR_VALUE,JSON_VALUE,IPV4_VALUE,IPV6_VALUE,BIN_VALUE",
				`sql_test,2026-07-10 17:10:20,3.142,-123,123,-1234,1234,-12345,12345,STR,"{""json"":true}",192.168.0.1,2001:db8::1,0x010203`,
				"", "",
			},
		},
		{
			Name: "SQL_markdown",
			Script: `
				SQL('select * from tag_data where name = ?', 'sql_test')
				MARKDOWN(timeformat('default'), tz('Local'))
			`,
			ExpectText: []string{
				"|NAME|TIME|VALUE|SHORT_VALUE|USHORT_VALUE|INT_VALUE|UINT_VALUE|LONG_VALUE|ULONG_VALUE|STR_VALUE|JSON_VALUE|IPV4_VALUE|IPV6_VALUE|BIN_VALUE|",
				"|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|",
				`|sql_test|2026-07-10 17:10:20|3.142000|-123|123|-1234|1234|-12345|12345|STR|{"json":true}|192.168.0.1|2001:db8::1|0x010203|`,
				"",
			},
		},
		{
			Name: "SQL_json",
			Script: `
				SQL('select * from tag_data where name = ?', 'sql_test')
				JSON(timeformat('default'), tz('Local'))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, gjson.Get(result, "reason").String(), "success")
				columns := gjson.Get(result, "data.columns").String()
				require.Equal(t, `["NAME","TIME","VALUE","SHORT_VALUE","USHORT_VALUE","INT_VALUE","UINT_VALUE","LONG_VALUE","ULONG_VALUE","STR_VALUE","JSON_VALUE","IPV4_VALUE","IPV6_VALUE","BIN_VALUE"]`, columns)
				types := gjson.Get(result, "data.types").String()
				require.Equal(t, `["string","datetime","double","int16","uint16","int32","uint32","int64","uint64","string","json","ipv4","ipv6","binary"]`, types)
				values := gjson.Get(result, "data.rows").String()
				require.Equal(t, `[["sql_test","2026-07-10 17:10:20",3.142,-123,123,-1234,1234,-12345,12345,"STR","{\"json\":true}","192.168.0.1","2001:db8::1","0x010203"]]`, values)
			},
		},
		{
			Name: "SQL_ndjson",
			Script: `
				SQL('select * from tag_data where name = ?', 'sql_test')
				NDJSON(timeformat('default'), tz('Local'))
				`,
			ExpectFunc: func(t *testing.T, result string) {
				require.Equal(t, `{"NAME":"sql_test","TIME":"2026-07-10 17:10:20","VALUE":3.142,"SHORT_VALUE":-123,"USHORT_VALUE":123,"INT_VALUE":-1234,"UINT_VALUE":1234,"LONG_VALUE":-12345,"ULONG_VALUE":12345,"STR_VALUE":"STR","JSON_VALUE":"{\"json\":true}","IPV4_VALUE":"192.168.0.1","IPV6_VALUE":"2001:db8::1","BIN_VALUE":"0x010203"}`+"\n\n", result)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShow(t *testing.T) {
	spi.SetServerInfoProvider(func() map[string]any { return map[string]any{"purpose": "test"} })
	tests := []TqlTestCase{
		{
			Name: "SQL_show_wrong",
			Script: `
				SQL('show wrong')
				CSV(header(true))
			`,
			ExpectErr: `f(SQL) unsupported show command "wrong"`,
		},
		{
			Name: "SQL_show_info",
			Script: `
				SQL('show info')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"NAME,VALUE",
				"purpose,test",
				"", "",
			},
		},
		{
			Name: "SQL_show_license",
			Script: `
				SQL('show license')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.Equal(t, 2, len(lines), result)
				require.Equal(t, "ID,TYPE,CUSTOMER,PROJECT,COUNTRY_CODE,INSTALL_DATE,ISSUE_DATE,STATUS", lines[0])
				// "00000000,COMMUNITY,NONE,NONE,KR,2026-07-08 10:15:59,20991231,Valid",
				require.Regexp(t, regexp.MustCompile(`^[0-9]+,[A-Z]+,[A-Z0-9]+,[A-Z0-9]+,[A-Z]{2},[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2},[0-9]{8},[A-Za-z]+$`), lines[1])
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowPorts(t *testing.T) {
	spi.SetServerPortsProvider(func(svc string) ([]*model.ServicePort, error) {
		ret := []*model.ServicePort{}
		if svc == "" || svc == "http" {
			ret = append(ret, &model.ServicePort{Service: "http", Address: "tcp://127.0.0.1:5654"})
		}
		if svc == "" || svc == "mqtt" {
			ret = append(ret, &model.ServicePort{Service: "mqtt", Address: "tcp://127.0.0.1:1883"})
		}
		return ret, nil
	})
	tests := []TqlTestCase{
		{
			Name: "SQL_show_ports",
			Script: `
				SQL('show ports')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"PORT,ADDRESS",
				"http,tcp://127.0.0.1:5654",
				"mqtt,tcp://127.0.0.1:1883",
				"", "",
			},
		},
		{
			Name: "SQL_show_ports_mqtt",
			Script: `
				SQL('show ports mqtt')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"PORT,ADDRESS",
				"mqtt,tcp://127.0.0.1:1883",
				"", "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowUsers(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_users",
			Script: `
				SQL('show users')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"USER_ID,NAME",
				"1,SYS",
				"", "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowTables(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_tables",
			Script: `
				SQL('show tables')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 4)
				require.Equal(t, "DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG", lines[0])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,LOG_DATA,[0-9]+,Log,$`), lines[1])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_DATA,[0-9]+,Tag,$`), lines[2])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_SIMPLE,[0-9]+,Tag,$`), lines[3])
			},
		},
		{
			Name: "SQL_show_tables_all",
			Script: `
				SQL('show tables --all')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 4)
				require.Equal(t, "DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG", lines[0])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,LOG_DATA,[0-9]+,Log,$`), lines[1])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_DATA,[0-9]+,Tag,$`), lines[2])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_SIMPLE,[0-9]+,Tag,$`), lines[3])
				require.GreaterOrEqual(t, len(lines), 8)
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_DATA_DATA_0,[0-9]+,KeyValue,Data$`), lines[4])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_DATA_META,[0-9]+,Lookup,Meta$`), lines[5])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_SIMPLE_DATA_0,[0-9]+,KeyValue,Data$`), lines[6])
				require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_SIMPLE_META,[0-9]+,Lookup,Meta$`), lines[7])
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowTable(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_table_log_data",
			Script: `
				SQL('show table log_data')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"COLUMN,TYPE,LENGTH,FLAG,INDEX",
				"TIME,datetime,31,,",
				"SHORT_VALUE,short,6,,",
				"USHORT_VALUE,ushort,5,,",
				"INT_VALUE,integer,11,,",
				"UINT_VALUE,uinteger,10,,",
				"LONG_VALUE,long,20,,",
				"ULONG_VALUE,ulong,20,,",
				"DOUBLE_VALUE,double,17,,",
				"FLOAT_VALUE,float,17,,",
				"STR_VALUE,varchar,400,,",
				"JSON_VALUE,json,32767,,",
				"IPV4_VALUE,ipv4,15,,",
				"IPV6_VALUE,ipv6,45,,",
				"TEXT_VALUE,text,67108864,,",
				"BIN_VALUE,binary,67108864,,",
				"", "",
			},
		},
		{
			Name: "SQL_show_table_log_data_all",
			Script: `
				SQL('show table log_data --all')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"COLUMN,TYPE,LENGTH,FLAG,INDEX",
				"_ARRIVAL_TIME,datetime,31,,",
				"TIME,datetime,31,,",
				"SHORT_VALUE,short,6,,",
				"USHORT_VALUE,ushort,5,,",
				"INT_VALUE,integer,11,,",
				"UINT_VALUE,uinteger,10,,",
				"LONG_VALUE,long,20,,",
				"ULONG_VALUE,ulong,20,,",
				"DOUBLE_VALUE,double,17,,",
				"FLOAT_VALUE,float,17,,",
				"STR_VALUE,varchar,400,,",
				"JSON_VALUE,json,32767,,",
				"IPV4_VALUE,ipv4,15,,",
				"IPV6_VALUE,ipv6,45,,",
				"TEXT_VALUE,text,67108864,,",
				"BIN_VALUE,binary,67108864,,",
				"_RID,long,20,,",
				"", "",
			},
		},
		{
			Name: "SQL_desc_tag_data",
			Script: `
				SQL('desc tag_data')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"COLUMN,TYPE,LENGTH,FLAG,INDEX",
				"NAME,varchar,100,tag name,",
				"TIME,datetime,31,base time,",
				"VALUE,double,17,summarized,",
				"SHORT_VALUE,short,6,,",
				"USHORT_VALUE,ushort,5,,",
				"INT_VALUE,integer,11,,",
				"UINT_VALUE,uinteger,10,,",
				"LONG_VALUE,long,20,,",
				"ULONG_VALUE,ulong,20,,",
				"STR_VALUE,varchar,400,,",
				"JSON_VALUE,json,32767,,",
				"IPV4_VALUE,ipv4,15,,",
				"IPV6_VALUE,ipv6,45,,",
				"BIN_VALUE,binary,32767,,",
				"", "",
			},
		},
		{
			Name: "SQL_describe_tag_data_all",
			Script: `
				SQL('describe tag_data --all')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"COLUMN,TYPE,LENGTH,FLAG,INDEX",
				"NAME,varchar,100,tag name,",
				"TIME,datetime,31,base time,",
				"VALUE,double,17,summarized,",
				"SHORT_VALUE,short,6,,",
				"USHORT_VALUE,ushort,5,,",
				"INT_VALUE,integer,11,,",
				"UINT_VALUE,uinteger,10,,",
				"LONG_VALUE,long,20,,",
				"ULONG_VALUE,ulong,20,,",
				"STR_VALUE,varchar,400,,",
				"JSON_VALUE,json,32767,,",
				"IPV4_VALUE,ipv4,15,,",
				"IPV6_VALUE,ipv6,45,,",
				"BIN_VALUE,binary,32767,,",
				"_RID,long,20,,",
				"", "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowIndexes(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_indexes",
			Script: `
				SQL('show indexes')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 5)
				require.Equal(t, "ID,DATABASE,USER,TABLE,COLUMN,INDEX_NAME,INDEX_TYPE,KEY_COMPRESS,MAX_LEVEL,PART_VALUE_COUNT,BITMAP_ENCODE", lines[0])

				required := map[string]struct {
					table  string
					column string
				}{
					"__PK_IDX__TAG_DATA_META_1":   {table: "_TAG_DATA_META", column: "_ID"},
					"_TAG_DATA_META_NAME":         {table: "_TAG_DATA_META", column: "NAME"},
					"__PK_IDX__TAG_SIMPLE_META_1": {table: "_TAG_SIMPLE_META", column: "_ID"},
					"_TAG_SIMPLE_META_NAME":       {table: "_TAG_SIMPLE_META", column: "NAME"},
				}
				seen := map[string]bool{}
				for _, line := range lines[1:] {
					fields := strings.Split(line, ",")
					require.GreaterOrEqual(t, len(fields), 11)
					idxName := fields[5]
					req, ok := required[idxName]
					if !ok {
						continue
					}
					require.Equal(t, "MACHBASEDB", fields[1])
					require.Equal(t, "SYS", fields[2])
					require.Equal(t, req.table, fields[3])
					require.Equal(t, req.column, fields[4])
					require.Equal(t, "REDBLACK", fields[6])
					seen[idxName] = true
				}
				for name := range required {
					require.True(t, seen[name], "required index missing: %s", name)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowIndex(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_index",
			Script: `
				SQL('show index _TAG_DATA_META_NAME')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"ID,TABLE,COLUMN,INDEX_NAME,INDEX_TYPE,KEY_COMPRESS,MAX_LEVEL,PART_VALUE_COUNT,BITMAP_ENCODE",
				"0,_TAG_DATA_META,NAME,_TAG_DATA_META_NAME,REDBLACK,UNCOMPRESSED,0,100000,EQUAL",
				"", "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

// TODO: Implement show indexgap test case. It is not clear what the expected output should be.
func TestTqlSqlShowIndexGap(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_indexgap",
			Script: `
				SQL('show indexgap')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"INDEX_ID,TABLE_NAME,INDEX_NAME,GAP",
				"", "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

// TODO: Implement show lsm test case. It is not clear what the expected output should be.
func TestTqlSqlShowLsm(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_lsm",
			Script: `
				SQL('show lsm')
				CSV(header(true))
			`,
			ExpectCSV: []string{
				"TABLE_NAME,INDEX_NAME,LEVEL,COUNT",
				"", "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowTags(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_insert",
			Script: `
				SCRIPT({$.yield('show_test', 1.234)})
				SQL('insert into tag_data (name,time,value) values(?,now,?)', value(0), value(1))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, gjson.Get(result, "data.message").String(), "a row inserted.")
			},
		},
		{
			Name: "SQL_exec_flush_table",
			Script: `
				FAKE(once(1))
				SQL('exec table_flush(tag_data)')
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, gjson.Get(result, "data.message").String(), "executed.")
			},
		},
		{
			Name: "SQL_show_tags_no_args",
			Script: `
				SQL('show tags')
				CSV(header(true))
			`,
			ExpectErr: `f(SQL) show tags expects at least 1 argument, got 0`,
		},
		{
			Name: "SQL_show_tags",
			Script: `
				SQL('show tags tag_data')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 2)
				require.Equal(t, "_ID,NAME,ROW_COUNT,MIN_TIME,MAX_TIME,RECENT_ROW_TIME,MIN_VALUE,MIN_VALUE_TIME,MAX_VALUE,MAX_VALUE_TIME", lines[0])
				hasTag := false
				hasValue := false
				for _, line := range lines[1:] {
					if strings.Contains(line, "show_test") {
						hasTag = true
					}
					if strings.Contains(line, "1.234") {
						hasValue = true
					}
				}
				require.True(t, hasTag, "expected to find tag 'show_test' in output")
				require.True(t, hasValue, "expected to find value '1.234' in output")
			},
		},
		{
			Name: "SQL_show_tags_log_table",
			Script: `
				SQL('show tags log_data')
				CSV(header(true))
			`,
			ExpectErr: `f(SQL) table "LOG_DATA" is not a tag table`,
		},
		{
			Name: "SQL_show_tagindexgap",
			Script: `
				SQL('show tagindexgap')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 1)
				require.Equal(t, "TABLE_ID,TABLE_NAME,STATUS,DISK_GAP,MEMORY_GAP", lines[0])
			},
		},
		{
			Name: "SQL_show_rollupgap",
			Script: `
				SQL('show rollupgap')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 1)
				// TODO: The expected output format for "show rollupgap" is not clear. Adjust the test as needed.
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowSessions(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_sessions",
			Script: `
				SQL('show sessions')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 2)
				require.Equal(t, "ID,USER_NAME,USER_ID,LOGIN_TIME,TYPE,USER_IP,MAX_QPX_MEM", lines[0])
				require.Regexp(t, regexp.MustCompile(`^[0-9]+,[A-Z]+,[0-9]+,[0-9]+,CLI,127.0.0.1,[0-9]+([.][0-9]+)?[KMG]?B$`), lines[1])
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowStatements(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_statements",
			Script: `
				SQL('show statements')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 2)
				require.Equal(t, "ID,SESSION_ID,STATE,RECORD_SIZE,QUERY", lines[0])
				require.Regexp(t, regexp.MustCompile(`^[0-9]+,[0-9]+,.+,[0-9]+,.+$`), lines[1])
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowStorage(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_storage",
			Script: `
				SQL('show storage')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 2)
				require.Equal(t, "TABLE_NAME,DATA_SIZE,INDEX_SIZE,TOTAL_SIZE", lines[0])
				// LOG_DATA,0,0,0
				require.Regexp(t, regexp.MustCompile(`[A-Z_]+,[0-9]+,[0-9]+,[0-9]+$`), lines[1])
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTqlSqlShowTableUsage(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show_table_usage",
			Script: `
				SQL('show table-usage')
				CSV(header(true))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
				require.GreaterOrEqual(t, len(lines), 2)
				require.Equal(t, "TABLE_NAME,STORAGE_USAGE", lines[0])
				// LOG_DATA,0,0,0
				require.Regexp(t, regexp.MustCompile(`^.+,[0-9]+$`), lines[1])
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
