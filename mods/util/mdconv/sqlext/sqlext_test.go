package sqlext_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/mdconv"
	"github.com/machbase/neo-server/v8/mods/util/mdconv/sqlext"
	"github.com/stretchr/testify/require"
)

func TestSqlFenceRendersAsCodeBlockByDefault(t *testing.T) {
	src := strings.Join([]string{
		"```sql",
		"select 1;",
		"```",
	}, "\n")
	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)
	result := out.String()
	require.NotContains(t, result, `class="sqlext"`)
	require.Contains(t, result, "<pre")
}

func TestSqlExecuteRendersResultBlock(t *testing.T) {
	src := strings.Join([]string{
		"```sql {execute=true}",
		"select 1 as v;",
		"```",
	}, "\n")
	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)
	result := out.String()
	require.Contains(t, result, `class="sqlext"`)
	require.Contains(t, result, `class="sqlext-result"`)
}

func TestSqlSourceOptionRendersSourcePreview(t *testing.T) {
	src := strings.Join([]string{
		"```sql {execute=true,source=show}",
		"select 1;",
		"```",
	}, "\n")
	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)
	result := out.String()
	require.Contains(t, result, "select 1;")
}

func TestParseOptionsUsesFormatAndHeaderAliases(t *testing.T) {
	opts := sqlext.ParseOptions(`{execute=true,format=csv,header=skip,p=[1,"neo"]}`)
	require.True(t, opts.Execute)
	require.Equal(t, "csv", opts.Format)
	require.Equal(t, "skip", opts.Header)
	require.Len(t, opts.Params, 2)
}

func TestParseOptionsUsesAllToDisablePreviewCompaction(t *testing.T) {
	opts := sqlext.ParseOptions(`{execute=true,preview=all}`)
	require.Equal(t, 0, opts.Preview)
}
