package http

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/http")
		o := module.Get("exports").(*js.Object)
		// http.request("http://host:port/path", {method: "GET"})
		o.Set("request", request(ctx, rt))
	}
}

type RequestConfig struct {
	Url     string            `json:"-"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

func request(ctx context.Context, rt *js.Runtime) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		config := RequestConfig{
			Method:  "GET",
			Headers: make(map[string]string),
			Body:    nil,
		}
		if len(call.Arguments) == 0 {
			panic("http.request requires at least 2 arguments")
		}
		config.Url = call.Arguments[0].String()
		if len(call.Arguments) > 1 {
			if err := rt.ExportTo(call.Arguments[1], &config); err != nil {
				panic(rt.ToValue("http.request invalid config: " + err.Error()))
			}
		}
		ret := rt.NewObject()
		ret.Set("url", config.Url)
		ret.Set("method", config.Method)
		ret.Set("do", do(ctx, rt, config))
		return ret
	}
}

func do(_ context.Context, rt *js.Runtime, reqConf RequestConfig) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		if len(call.Arguments) == 0 {
			panic("http.request.do requires at least 1 argument")
		}
		var callback js.Callable
		if err := rt.ExportTo(call.Arguments[0], &callback); err != nil {
			panic(rt.ToValue("http.request.do invalid callback: " + err.Error()))
		}

		var reqBody io.Reader
		switch v := reqConf.Body.(type) {
		case string:
			reqBody = strings.NewReader(v)
		default:
			reqBody = nil
		}

		responseObj := rt.NewObject()
		httpClient := http.DefaultClient
		httpRequest, httpErr := http.NewRequest(strings.ToUpper(reqConf.Method), reqConf.Url, reqBody)
		var httpResponse *http.Response
		if httpErr == nil {
			for k, v := range reqConf.Headers {
				httpRequest.Header.Set(k, v)
			}
			if reqConf.Method == "POST" || reqConf.Method == "PUT" {
				httpRequest.Body = io.NopCloser(reqBody)
			}
			if rsp, err := httpClient.Do(httpRequest); err != nil {
				httpErr = err
			} else {
				defer rsp.Body.Close()
				httpResponse = rsp
				responseObj.Set("status", rsp.StatusCode)
				responseObj.Set("statusText", rsp.Status)
				hdr := map[string]any{}
				for k, v := range rsp.Header {
					if len(v) == 1 {
						hdr[k] = v[0]
					} else {
						hdr[k] = v
					}
				}
				// TODO: implement get(), forEach(), has(), keys()
				responseObj.Set("headers", hdr)
			}
		}
		responseObj.Set("url", reqConf.Url)
		responseObj.Set("error", responseError(httpErr, rt))
		responseObj.Set("text", responseText(httpResponse, rt))
		responseObj.Set("blob", responseBlob(httpResponse, rt))
		responseObj.Set("json", responseJson(httpResponse, rt))
		responseObj.Set("csv", responseCsv(httpResponse, rt))

		if _, e := callback(js.Undefined(), responseObj); e != nil {
			return rt.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
		}
		return js.Undefined()
	}
}

func responseError(httpErr error, rt *js.Runtime) func(js.FunctionCall) js.Value {
	return func(_ js.FunctionCall) js.Value {
		if httpErr == nil {
			return js.Null()
		}
		return rt.ToValue(httpErr.Error())
	}
}

func responseText(httpResponse *http.Response, rt *js.Runtime) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		if b, err := io.ReadAll(httpResponse.Body); err == nil {
			return rt.ToValue(string(b))
		} else {
			return rt.ToValue(err.Error())
		}
	}
}

func responseBlob(httpResponse *http.Response, rt *js.Runtime) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		if b, err := io.ReadAll(httpResponse.Body); err == nil {
			return rt.ToValue(rt.NewArrayBuffer(b))
		} else {
			return rt.ToValue(err.Error())
		}
	}
}

func responseJson(httpResponse *http.Response, rt *js.Runtime) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		dec := json.NewDecoder(httpResponse.Body)
		data := map[string]any{}
		err := dec.Decode(&data)
		if err == io.EOF {
			return js.Null()
		} else if err != nil {
			return rt.ToValue(err.Error())
		}
		return rt.ToValue(data)
	}
}

func responseCsv(httpResponse *http.Response, rt *js.Runtime) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		dec := csv.NewReader(httpResponse.Body)
		dec.FieldsPerRecord = -1
		dec.TrimLeadingSpace = true
		dec.ReuseRecord = true
		arr := []any{}
		for {
			row, err := dec.Read()
			if err == io.EOF {
				break
			} else if err != nil {
				return rt.NewGoError(fmt.Errorf("HTTPError %s", err.Error()))
			}
			s := make([]any, len(row))
			for i, v := range row {
				s[i] = v
			}
			arr = append(arr, rt.NewArray(s...))
		}
		return rt.ToValue(rt.NewArray(arr...))
	}
}
