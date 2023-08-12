package runes

import (
	"reflect"
	"testing"

	"github.com/d5/tengo/v2/require"
)

type twidth struct {
	r      []rune
	length int
}

func TestRuneWidth(t *testing.T) {
	runes := []twidth{
		{[]rune("☭"), 1},
		{[]rune("a"), 1},
		{[]rune("你"), 2},
		{[]rune("속"), 2},
		{[]rune(""), 0},
		{ColorFilter([]rune("☭\033[13;1m你")), 3},
	}
	for _, r := range runes {
		if w := WidthAll(r.r); w != r.length {
			t.Fatal("result not expect", r.r, r.length, w)
		}
	}
}

type tagg struct {
	r      [][]rune
	e      [][]rune
	length int
}

func TestAggRunes(t *testing.T) {
	runes := []tagg{
		{
			[][]rune{[]rune("ab"), []rune("a"), []rune("abc")},
			[][]rune{[]rune("b"), []rune(""), []rune("bc")},
			1,
		},
		{
			[][]rune{[]rune("addb"), []rune("ajkajsdf"), []rune("aasdfkc")},
			[][]rune{[]rune("ddb"), []rune("jkajsdf"), []rune("asdfkc")},
			1,
		},
		{
			[][]rune{[]rune("ddb"), []rune("ajksdf"), []rune("aasdfkc")},
			[][]rune{[]rune("ddb"), []rune("ajksdf"), []rune("aasdfkc")},
			0,
		},
		{
			[][]rune{[]rune("ddb"), []rune("ddajksdf"), []rune("ddaasdfkc")},
			[][]rune{[]rune("b"), []rune("ajksdf"), []rune("aasdfkc")},
			2,
		},
	}
	for _, r := range runes {
		same, off := Aggregate(r.r)
		if off != r.length {
			t.Fatal("result not expect", off)
		}
		if len(same) != off {
			t.Fatal("result not expect", same)
		}
		if !reflect.DeepEqual(r.r, r.e) {
			t.Fatal("result not expect")
		}
	}
}

func TestEqual(t *testing.T) {
	b := Equal([]rune("fedcba"), []rune("fedcb"))
	require.False(t, b)

	b = Equal([]rune("fedcba"), []rune("fedcbz"))
	require.False(t, b)

	b = Equal([]rune("fedcba"), []rune("fedcba"))
	require.True(t, b)
}

func TestIndexAllBack(t *testing.T) {
	i := IndexAllBack([]rune("fedcba"), []rune("dc"))
	require.Equal(t, 2, i)

	i = IndexAllBack([]rune("fedcba"), []rune("dcx"))
	require.Equal(t, -1, i)
}

func TestIndexAll(t *testing.T) {
	i := IndexAll([]rune("fedcba"), []rune("dc"))
	require.Equal(t, 2, i)

	i = IndexAll([]rune("fedcba"), []rune("dcx"))
	require.Equal(t, -1, i)
}

func TestHasPrefix(t *testing.T) {
	b := HasPrefix([]rune("fedcba"), []rune("ed"))
	require.False(t, b)

	b = HasPrefix([]rune("fedcba"), []rune("fed"))
	require.True(t, b)
}
