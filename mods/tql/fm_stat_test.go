package tql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatHelpers(t *testing.T) {
	node := &Node{}

	require.Equal(t, StatCategoryName("alpha"), node.fmCategory("alpha"))
	name := "beta"
	require.Equal(t, StatCategoryName("beta"), node.fmCategory(&name))
	require.Equal(t, "123", node.fmCategory(123))

	bins, err := node.fmBins(0, 10, 2)
	require.NoError(t, err)
	require.Equal(t, "[0.000000-10.000000)/2.000000", bins.(*HistogramStepBins).String())

	maxBins, err := node.fmBins(7)
	require.NoError(t, err)
	require.Equal(t, HistogramMaxBins(7), maxBins)

	_, err = node.fmBins()
	require.EqualError(t, err, "f(bins) invalid number of args; expected 1 or 3, got 0")

	require.Equal(t, BoxplotFormat("chart"), node.fmBoxplotOutputFormat("CHART"))
	require.Equal(t, BoxplotFormat("standard"), node.fmBoxplotOutputFormat("unexpected"))
}

func TestHistogramOrder(t *testing.T) {
	hist := &HistogramPredictedBins{}

	hist.buckets = map[StatCategoryName]*HistogramBuckets{}
	hist.buckets["Cat.A"] = nil
	hist.buckets["Cat.B"] = nil
	hist.buckets["Cat.C"] = nil
	hist.buckets["Cat.D"] = nil

	result := hist.orderedCategoryNames()
	require.EqualValues(t, []StatCategoryName{"Cat.A", "Cat.B", "Cat.C", "Cat.D"}, result)

	hist.orders = []StatCategoryName{"Cat.D", "Cat.C", "Cat.B", "Cat.A"}
	result = hist.orderedCategoryNames()
	require.EqualValues(t, []StatCategoryName{"Cat.D", "Cat.C", "Cat.B", "Cat.A"}, result)

	hist.orders = []StatCategoryName{"Cat.D", "Cat.C"}
	result = hist.orderedCategoryNames()
	require.EqualValues(t, []StatCategoryName{"Cat.D", "Cat.C", "Cat.A", "Cat.B"}, result)

	hist.orders = []StatCategoryName{"Cat.D"}
	result = hist.orderedCategoryNames()
	require.EqualValues(t, []StatCategoryName{"Cat.D", "Cat.A", "Cat.B", "Cat.C"}, result)
}
