package tagql_test

import (
	"strings"
	"testing"

	"github.com/d5/tengo/v2/require"
	. "github.com/machbase/neo-server/mods/tagql"
)

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
		q:      "table/tag?range=1.0s&time=last",
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#val",
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#(val * 0.01)",
		expect: "SELECT time, (val * 0.01) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#(val + val2/2)?range=2.34s&time=last&limit=2000",
		expect: "SELECT time, (val + val2/2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 2000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#val?range=2.34s&time=now&limit=100",
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#val?range=2.34s&time=now&limit=100",
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#val?range=2.34s&time=123456789000",
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN 123456789000 - 2340000000 AND 123456789000 LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#avg(val1+val2)",
		expect: "SELECT time, avg(val1+val2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
}

func TestTagQLGroupBy(t *testing.T) {
	TagQLTestCase{
		q:      "table/tag#stddev(val)?range=3.45s&time=123456789000&group=1ms&limit=100",
		expect: "SELECT round(to_timestamp(time)/1000000)*1000000 time, stddev(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN 123456789000 - 3450000000 AND 123456789000 GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#stddev(val)?range=2.34s&time=last&group=0.5ms&limit=100",
		expect: "SELECT round(to_timestamp(time)/500000)*500000 time, stddev(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#stddev(val)?range=2.34s&time=now&group=0.5ms&limit=100",
		expect: "SELECT round(to_timestamp(time)/500000)*500000 time, stddev(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now GROUP BY time ORDER BY time LIMIT 100",
		err:    ""}.
		run(t)
}

func TestTagQLErrors(t *testing.T) {
	TagQLTestCase{q: "table.tag", expect: "", err: "invalid syntax"}.run(t)
	TagQLTestCase{q: "table/tag#f1(value)", expect: "", err: "undefined function f1"}.run(t)
	TagQLTestCase{q: "table/tag#value?range=1x", expect: "", err: "invalid range syntax"}.run(t)
	TagQLTestCase{q: "table/tag#value?group=1x", expect: "", err: "invalid group syntax"}.run(t)
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
	ql, err := ParseTagQL(tc.q)
	if len(tc.err) > 0 {
		require.NotNil(t, err)
		require.Equal(t, tc.err, err.Error())
		return
	}
	require.Nil(t, err, tc.q)
	require.NotNil(t, ql, tc.q)
	require.Equal(t, normalize(tc.expect), normalize(ql.ToSQL()), tc.q)
}
