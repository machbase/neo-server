package input

import (
	"reflect"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/stretchr/testify/require"
)

func gatherMeasureNames(g *metric.Gather) []string {
	measures := reflect.ValueOf(g).Elem().FieldByName("measures")
	names := make([]string, 0, measures.Len())
	for i := 0; i < measures.Len(); i++ {
		names = append(names, measures.Index(i).FieldByName("Name").String())
	}
	return names
}

func TestRuntimeGather(t *testing.T) {
	g := &metric.Gather{}

	err := (&Runtime{}).Gather(g)
	require.NoError(t, err)

	names := gatherMeasureNames(g)
	require.Contains(t, names, "runtime:goroutines")
	require.Contains(t, names, "runtime:heap_inuse")
	require.Contains(t, names, "runtime:cgo_call")
	require.Contains(t, names, "ps:cpu_percent")
	require.Contains(t, names, "ps:mem_percent")
}

func TestNetstatGather(t *testing.T) {
	g := &metric.Gather{}

	err := (&Netstat{}).Gather(g)
	if err != nil {
		require.Contains(t, err.Error(), "failed to collect netstat")
		return
	}

	names := gatherMeasureNames(g)
	for _, name := range netStatzList {
		require.Contains(t, names, "netstat:"+name)
	}
}
