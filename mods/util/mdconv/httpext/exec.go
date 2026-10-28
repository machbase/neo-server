package httpext

import "github.com/machbase/neo-server/v8/mods/util/httpdsl"

func executeRawHTTPClient(content string, options ...httpdsl.Option) (string, string, error) {
	exchange, err := httpdsl.Execute(content, options...)
	return exchange.RequestRaw, exchange.ResponseRaw, err
}
