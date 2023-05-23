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
		q:      "example/sig.1",
		expect: "SELECT time, value FROM EXAMPLE WHERE name = 'sig.1' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$EXAMPLE_STAT WHERE name = 'sig.1') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'sig.1') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#val",
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#val?range=2.34s&time=last&limit=2000",
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 2000",
		err:    ""}.
		run(t)
	TagQLTestCase{
		q:      "table/tag#val?range=2.34s&time=now&limit=100",
		expect: "SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN now - 2340000000 AND now LIMIT 100",
		err:    ""}.
		run(t)
		// TODO it should be 'SELECT val1+val2 FROM...'
	TagQLTestCase{
		q:      "table/tag#(val1+val2)",
		expect: "SELECT time, val1, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME - 1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10000",
		err:    ""}.
		run(t)
}

func TestTagQLErrors(t *testing.T) {
	TagQLTestCase{q: "table.tag", expect: "", err: "invalid syntax"}.run(t)
	TagQLTestCase{q: "table/tag#f1(value)", expect: "", err: "undefined function f1"}.run(t)
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
