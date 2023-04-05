package csv

import (
	"bytes"
	"encoding/csv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCsvDecoder(t *testing.T) {
	data := []byte("json.data,1678147077000000000,1.234,typeval,1234,1234,1111,pnameval,1,\"{\"\"name\"\":1234}\"")

	r := csv.NewReader(bytes.NewBuffer(data))
	r.Comma = ','

	fields, err := r.Read()
	if err != nil {
		panic(err)
	}
	require.Equal(t, "json.data", fields[0])
	require.Equal(t, "1678147077000000000", fields[1])
	require.Equal(t, "1.234", fields[2])
	require.Equal(t, "typeval", fields[3])
	require.Equal(t, "1234", fields[4])
	require.Equal(t, "1234", fields[5])
	require.Equal(t, "1111", fields[6])
	require.Equal(t, "pnameval", fields[7])
	require.Equal(t, "1", fields[8])
	require.Equal(t, "{\"name\":1234}", fields[9])
}
