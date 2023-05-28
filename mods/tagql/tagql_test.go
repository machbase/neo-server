package tagql_test

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/d5/tengo/v2/require"
	. "github.com/machbase/neo-server/mods/tagql"
)

func TestTagQLFile(t *testing.T) {
	r, err := os.Open("./test/simple.tql")
	require.Nil(t, err)
	tql, err := Parse("table", "tag", r)
	require.Nil(t, err)
	require.NotNil(t, tql)
	require.Equal(t,
		normalize(`SELECT time, value FROM table
			WHERE
				name = 'tag'
			AND time
				BETWEEN
					(SELECT MAX_TIME - 10000000000 FROM V$table_STAT WHERE name = 'tag')
				AND (SELECT MAX_TIME FROM V$table_STAT WHERE name = 'tag')
			LIMIT 1000000`),
		normalize(tql.ToSQL()), "./test/simple.tql")
}

type TagQLTestCase struct {
	q      string
	expect string
	err    string
}

func TestTagQLMajorParts(t *testing.T) {
	TagQLTestCase{
		q:      "table/tag",
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('val')`),
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('value', range('last', '1.0s'))`),
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('value', range('last', '12.0s'))`),
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 12000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('val1', 'val2')`),
		expect: "SELECT time, val1, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('(val * 0.01) altVal', 'val2')`),
		expect: "SELECT time, (val * 0.01) altVal, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('(val + val2/2)', range('last', '2.34s'), limit(2000))`),
		expect: "SELECT time, (val + val2/2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 2000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('val', range('now', '2.34s'), limit(100))`),
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('value', range(123456789000, '2.34s'))`),
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN 123456789000 - 2340000000 AND 123456789000 LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('AVG(val1+val2)')`),
		expect: "SELECT time, AVG(val1+val2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
}

func TestTagQLMap(t *testing.T) {
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('val1', range('last', '1s'))`) + "&map=" + url.QueryEscape(`MODTIME(K, V, '100ms')`),
		expect: "SELECT time, val1 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
}

func TestTagQLGroupBy(t *testing.T) {
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('STDDEV(val)', range(123456789000, "3.45s", '1ms'), limit(100))`),
		expect: "SELECT from_timestamp(round(to_timestamp(time)/1000000)*1000000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN 123456789000 - 3450000000 AND 123456789000 GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('STDDEV(val)', 'zval', range('last', '2.34s', '0.5ms'), limit(100))`),
		expect: "SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val), zval FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag?src=" + url.QueryEscape(`INPUT('STDDEV(val)', range('now', '2.34s', '0.5ms'), limit(100))`),
		expect: "SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
}

func TestTagQLErrors(t *testing.T) {
	TagQLTestCase{q: "table.tag", expect: "", err: "invalid syntax"}.run(t)
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
	ql, err := ParseURI(tc.q)
	if len(tc.err) > 0 {
		require.NotNil(t, err)
		require.Equal(t, tc.err, err.Error())
		return
	}
	require.Nil(t, err, tc.q)
	require.NotNil(t, ql, tc.q)
	require.Equal(t, normalize(tc.expect), normalize(ql.ToSQL()), tc.q)
}
