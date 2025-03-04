package tql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
