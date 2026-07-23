package httpext

import "github.com/machbase/neo-server/v8/mods/util/httpdsl"

func executeRawHTTPClient(content string) (string, string, error) {
	exchange, err := httpdsl.Execute(content)
	return exchange.RequestRaw, exchange.ResponseRaw, err
}
