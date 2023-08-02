package maps_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/tql"
)

func TestTagQLFile(t *testing.T) {
	text := `QUERY('value', between('last-10s', 'last'), from("table", "tag", "time"))`
	ret, err := tql.CompileSource(text, nil, nil)
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t,
		normalize(`SELECT time, value 
			FROM TABLE WHERE name = 'tag' 
			AND time BETWEEN 
					(SELECT MAX_TIME-10000000000 FROM V$TABLE_STAT WHERE name = 'tag') 
				AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag')
			LIMIT 0, 1000000`),
		normalize(ret.ToSQL()), "./test/simple.tql")
}

type TagQLTestCase struct {
	tq     string
	expect string
	err    string
}

func TestSqlBetween(t *testing.T) {
	TagQLTestCase{
		tq:     `QUERY( 'value', from('example', 'barn'), between('last -1h', 'last'))`,
		expect: `SELECT time, value FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-3600000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') LIMIT 0, 1000000`,
		err:    "",
	}.run(t)
	TagQLTestCase{
		tq:     `QUERY( 'value', from('example', 'barn'), between('last -1h23m45s', 'last'))`,
		expect: `SELECT time, value FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-5025000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') LIMIT 0, 1000000`,
		err:    "",
	}.run(t)
	TagQLTestCase{
		tq:     `QUERY( 'STDDEV(value)', from('example', 'barn'), between('last -1h23m45s', 'last', '10m'))`,
		expect: `SELECT from_timestamp(round(to_timestamp(time)/600000000000)*600000000000) time, STDDEV(value) FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-5025000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') GROUP BY time ORDER BY time LIMIT 0, 1000000`,
		err:    "",
	}.run(t)
	TagQLTestCase{
		tq:     `QUERY( 'STDDEV(value)', from('example', 'barn'), between(1677646906*1000000000, 'last', '1s'))`,
		expect: `SELECT from_timestamp(round(to_timestamp(time)/1000000000)*1000000000) time, STDDEV(value) FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN 1677646906000000000 AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') GROUP BY time ORDER BY time LIMIT 0, 1000000`,
		err:    "",
	}.run(t)
}

func TestTagQLMajorParts(t *testing.T) {
	TagQLTestCase{
		tq:     `QUERY('value', from('table', 'tag'))`,
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('val', from('table', 'tag'))`,
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('value', from('table', 'tag'), between('last -1.0s', 'last'))`,
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('value', from('table', 'tag'), between('last-12.0s', 'last'))`,
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-12000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('val1', 'val2' , from('table', 'tag'))`,
		expect: "SELECT time, val1, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('(val * 0.01) altVal', 'val2', from('table', 'tag'))`,
		expect: "SELECT time, (val * 0.01) altVal, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('(val + val2/2)', from('table', 'tag'), between('last-2.34s', 'last'), limit(10, 2000))`,
		expect: "SELECT time, (val + val2/2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10, 2000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('val', from('table', 'tag'), between('now -2.34s', 'now'), limit(5, 100))`,
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (now-2340000000) AND now LIMIT 5, 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('value', from('table', 'tag'), between(123456789000-2.34*1000000000, 123456789000))`,
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN 121116789000 AND 123456789000 LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `QUERY('AVG(val1+val2)', from('table', 'tag'))`,
		expect: "SELECT time, AVG(val1+val2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
}

func TestTagQLMap(t *testing.T) {
	TagQLTestCase{
		tq:     `QUERY('val1', from('table', 'tag'), between('last-1s', 'last'))`,
		expect: "SELECT time, val1 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
}

func TestTagQLGroupBy(t *testing.T) {
	TagQLTestCase{
		tq:     `INPUT(QUERY('STDDEV(val)', from('table', 'tag'), between(123456789000 - 3.45*1000000000, 123456789000, '1ms'), limit(1, 100)))`,
		expect: "SELECT from_timestamp(round(to_timestamp(time)/1000000)*1000000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN 120006789000 AND 123456789000 GROUP BY time ORDER BY time LIMIT 1, 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('STDDEV(val)', 'zval', from('table', 'tag'), between('last-2.34s', 'last', '0.5ms'), limit(2, 100)))`,
		expect: "SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val), zval FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') GROUP BY time ORDER BY time LIMIT 2, 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('STDDEV(val)', from('table', 'tag'), between('now-2.34s', 'now', '0.5ms'), limit(3, 100)))`,
		expect: "SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN (now-2340000000) AND now GROUP BY time ORDER BY time LIMIT 3, 100",
		err:    ""}.
		run(t)
}

func normalize(ret string) string {
	lines := []string{}
	for _, str := range strings.Split(ret, "\n") {
		l := strings.TrimSpace(str)
		if l == "" {
			continue
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, " ")
}

func (tc TagQLTestCase) run(t *testing.T) {
	ret, err := tql.CompileSource(tc.tq, nil, nil)
	if err != nil {
		t.Fatalf("tq:'%s' parse err:%s", tc.tq, err.Error())
	}
	require.NotNil(t, ret)
	if len(tc.err) > 0 {
		require.NotNil(t, err)
		require.Equal(t, tc.err, err.Error())
		return
	}
	msg := fmt.Sprintf("%v", tc.tq)
	require.Nil(t, err, msg)
	require.NotNil(t, ret, msg)
	require.Equal(t, normalize(tc.expect), normalize(ret.ToSQL()), msg)
}
