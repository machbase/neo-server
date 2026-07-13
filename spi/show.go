package spi

import (
	"errors"
	"slices"

	"github.com/machbase/neo-client/api"
)

var serverInfoProvider func() map[string]any

func SetDefaultServerInfo(provider func() map[string]any) {
	serverInfoProvider = provider
}

type ShowInfoResultSet struct {
	ResultSetBase
	keys []string
	data map[string]any
}

var _ ResultSet = (*ShowInfoResultSet)(nil)

func (si *ShowInfoResultSet) Columns() api.Columns {
	return api.Columns{
		api.MakeColumnString("NAME"),
		api.MakeColumnAny("VALUE"),
	}
}

func (si *ShowInfoResultSet) Iter(callback func(values []interface{}) bool) {
	if si.err != nil {
		return
	}

	for _, k := range si.keys {
		v := si.data[k]
		if !callback([]interface{}{k, v}) {
			return
		}
	}
}

func ShowInfo() *ShowInfoResultSet {
	if serverInfoProvider == nil {
		return &ShowInfoResultSet{ResultSetBase: ResultSetBase{err: errors.New("server info provider is not set")}}
	}
	serverInfo := serverInfoProvider()
	keys := make([]string, 0, len(serverInfo))
	for k := range serverInfo {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return &ShowInfoResultSet{keys: keys, data: serverInfo}
}
