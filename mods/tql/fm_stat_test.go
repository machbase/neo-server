package tql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHistogramOrder(t *testing.T) {
	hist := &Histogram{}

	hist.buckets = map[HistogramBucketName]*HistogramBuckets{}
	hist.buckets["Cat.A"] = nil
	hist.buckets["Cat.B"] = nil
	hist.buckets["Cat.C"] = nil
	hist.buckets["Cat.D"] = nil

	result := hist.orderedCategoryNames()
	require.EqualValues(t, []HistogramBucketName{"Cat.A", "Cat.B", "Cat.C", "Cat.D"}, result)

	hist.orders = []HistogramBucketName{"Cat.D", "Cat.C", "Cat.B", "Cat.A"}
	result = hist.orderedCategoryNames()
	require.EqualValues(t, []HistogramBucketName{"Cat.D", "Cat.C", "Cat.B", "Cat.A"}, result)

	hist.orders = []HistogramBucketName{"Cat.D", "Cat.C"}
	result = hist.orderedCategoryNames()
	require.EqualValues(t, []HistogramBucketName{"Cat.D", "Cat.C", "Cat.A", "Cat.B"}, result)

	hist.orders = []HistogramBucketName{"Cat.D"}
	result = hist.orderedCategoryNames()
	require.EqualValues(t, []HistogramBucketName{"Cat.D", "Cat.A", "Cat.B", "Cat.C"}, result)
}
