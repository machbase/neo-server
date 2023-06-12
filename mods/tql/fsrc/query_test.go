package fsrc

import (
	"fmt"
	"strings"
	"testing"

	"github.com/d5/tengo/v2/require"
)

func TestTagQLFile(t *testing.T) {
	text := `INPUT( QUERY('value', range('last', '10s'), from("table", "tag", "time")) )`
	expr, err := Parse(text)
	require.Nil(t, err)
	require.NotNil(t, expr)
	ret, err := expr.Eval(nil)
	in := ret.(*input)
	src := in.dbSrc
	require.Nil(t, err)
	require.NotNil(t, ret)
	require.Equal(t,
		normalize(`SELECT time, value FROM TABLE
			WHERE
				name = 'tag'
			AND time
				BETWEEN
					(SELECT MAX_TIME - 10000000000 FROM V$TABLE_STAT WHERE name = 'tag')
				AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag')
			LIMIT 0, 1000000`),
		normalize(src.ToSQL()), "./test/simple.tql")
}

type TagQLTestCase struct {
	tq     string
	expect string
	err    string
}

func TestTagQLMajorParts(t *testing.T) {
	TagQLTestCase{
		tq:     `INPUT(QUERY('value', from('table', 'tag')))`,
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('val', from('table', 'tag')))`,
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('value', from('table', 'tag'), range('last', '1.0s')))`,
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('value', from('table', 'tag'), range('last', '12.0s')))`,
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 12000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('val1', 'val2' , from('table', 'tag')))`,
		expect: "SELECT time, val1, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('(val * 0.01) altVal', 'val2', from('table', 'tag')))`,
		expect: "SELECT time, (val * 0.01) altVal, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('(val + val2/2)', from('table', 'tag'), range('last', '2.34s'), limit(10, 2000)))`,
		expect: "SELECT time, (val + val2/2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10, 2000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('val', from('table', 'tag'), range('now', '2.34s'), limit(5, 100)))`,
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now LIMIT 5, 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('value', from('table', 'tag'), range(123456789000, '2.34s')))`,
		expect: "SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN 123456789000 - 2340000000 AND 123456789000 LIMIT 0, 1000000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('AVG(val1+val2)', from('table', 'tag')))`,
		expect: "SELECT time, AVG(val1+val2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
}

func TestTagQLMap(t *testing.T) {
	TagQLTestCase{
		tq:     `INPUT(QUERY('val1', from('table', 'tag'), range('last', '1s')))`,
		expect: "SELECT time, val1 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000",
		err:    ""}.
		run(t)
}

func TestTagQLGroupBy(t *testing.T) {
	TagQLTestCase{
		tq:     `INPUT(QUERY('STDDEV(val)', from('table', 'tag'), range(123456789000, "3.45s", '1ms'), limit(1, 100)))`,
		expect: "SELECT from_timestamp(round(to_timestamp(time)/1000000)*1000000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN 123456789000 - 3450000000 AND 123456789000 GROUP BY time ORDER BY time LIMIT 1, 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('STDDEV(val)', 'zval', from('table', 'tag'), range('last', '2.34s', '0.5ms'), limit(2, 100)))`,
		expect: "SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val), zval FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') GROUP BY time ORDER BY time LIMIT 2, 100",
		err:    ""}.
		run(t)
	TagQLTestCase{
		tq:     `INPUT(QUERY('STDDEV(val)', from('table', 'tag'), range('now', '2.34s', '0.5ms'), limit(3, 100)))`,
		expect: "SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now GROUP BY time ORDER BY time LIMIT 3, 100",
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
	expr, err := Parse(tc.tq)
	if err != nil {
		t.Fatalf("tq:'%s' parse err:%s", tc.tq, err.Error())
	}
	ret, err := expr.Eval(nil)
	if err != nil {
		t.Fatalf("tq:'%s' eval err:%s", tc.tq, err.Error())
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
	in := ret.(*input)
	require.Equal(t, normalize(tc.expect), normalize(in.dbSrc.ToSQL()), msg)
}
