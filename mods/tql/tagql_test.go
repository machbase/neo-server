package tql_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/d5/tengo/v2/require"
	. "github.com/machbase/neo-server/mods/tql"
)

func TestTagQLFile(t *testing.T) {
	text := `
#
# tql example
#

INPUT( QUERY('value', range('last', '10s'), from("table", "tag", "time")) )
    
OUTPUT(
        'csv',
        timeformat('ns'), 
        heading(true)
    )
`
	r := bytes.NewBuffer([]byte(text))
	tql, err := Parse(r)
	require.Nil(t, err)
	require.NotNil(t, tql)
	require.Equal(t,
		normalize(`SELECT time, value FROM TABLE
			WHERE
				name = 'tag'
			AND time
				BETWEEN
					(SELECT MAX_TIME - 10000000000 FROM V$TABLE_STAT WHERE name = 'tag')
				AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag')
			LIMIT 1000000`),
		normalize(tql.ToSQL()), "./test/simple.tql")
}

type TagQLTestCase struct {
	tq     []string
	expect string
	err    string
}

func TestTagQLMajorParts(t *testing.T) {
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('value', from('table', 'tag')))`, `OUTPUT(CSV())`},
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('val', from('table', 'tag')))`, `OUTPUT(CSV())`},
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('value', from('table', 'tag'), range('last', '1.0s')))`, `OUTPUT(CSV())`},
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('value', from('table', 'tag'), range('last', '12.0s')))`, `OUTPUT(CSV())`},
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 12000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('val1', 'val2' , from('table', 'tag')))`, `OUTPUT(CSV())`},
		expect: "SELECT time, val1, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('(val * 0.01) altVal', 'val2', from('table', 'tag')))`, `OUTPUT(CSV())`},
		expect: "SELECT time, (val * 0.01) altVal, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('(val + val2/2)', from('table', 'tag'), range('last', '2.34s'), limit(2000)))`, `OUTPUT(CSV())`},
		expect: "SELECT time, (val + val2/2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 2000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('val', from('table', 'tag'), range('now', '2.34s'), limit(100)))`, `OUTPUT(CSV())`},
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('value', from('table', 'tag'), range(123456789000, '2.34s')))`, `OUTPUT(CSV())`},
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN 123456789000 - 2340000000 AND 123456789000 LIMIT 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('AVG(val1+val2)', from('table', 'tag')))`, `OUTPUT(CSV())`},
		expect: "SELECT time, AVG(val1+val2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 1000000",
		err:    ""}.
		run(t)
}

func TestTagQLMap(t *testing.T) {
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('val1', from('table', 'tag'), range('last', '1s')))`, `PUSHKEY(roundTime(K, '100ms'))`, `OUTPUT(CSV())`},
		expect: "SELECT time, val1 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 1000000",
		err:    ""}.
		run(t)
}

func TestTagQLGroupBy(t *testing.T) {
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('STDDEV(val)', from('table', 'tag'), range(123456789000, "3.45s", '1ms'), limit(100)))`, `OUTPUT(CSV())`},
		expect: "SELECT from_timestamp(round(to_timestamp(time)/1000000)*1000000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN 123456789000 - 3450000000 AND 123456789000 GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('STDDEV(val)', 'zval', from('table', 'tag'), range('last', '2.34s', '0.5ms'), limit(100)))`, `OUTPUT(CSV())`},
		expect: "SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val), zval FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     []string{`INPUT(QUERY('STDDEV(val)', from('table', 'tag'), range('now', '2.34s', '0.5ms'), limit(100)))`, `OUTPUT(CSV())`},
		expect: "SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
}

func TestTagQLErrors(t *testing.T) {
	// TagQLTestCase{tq: {`INPUT(X(table.tag"}, expect: "", err: "invalid syntax"}.run(t)
	// TagQLTestCase{q: "table/tag?field=" + url.QueryEscape("val1 +* val2"), expect: "", err: "invalid token: '+*'"}.run(t)
	// TagQLTestCase{q: "table/tag?field=" + url.QueryEscape("f1(value)"), expect: "", err: "undefined function f1"}.run(t)
	// TagQLTestCase{q: "table/tag?field=value&range=1x", expect: "", err: "invalid range syntax"}.run(t)
	//	TagQLTestCase{q: "table/tagvalue?group=1x", expect: "", err: "invalid group syntax"}.run(t)
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
	ql, err := ParseContext(context.TODO(), map[string][]string{"_tq": tc.tq})
	if len(tc.err) > 0 {
		require.NotNil(t, err)
		require.Equal(t, tc.err, err.Error())
		return
	}
	msg := fmt.Sprintf("%v", tc.tq)
	require.Nil(t, err, msg)
	require.NotNil(t, ql, msg)
	require.Equal(t, normalize(tc.expect), normalize(ql.ToSQL()), msg)
}
