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
)

func new_client(ctx context.Context, rt *js.Runtime) func(js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		ret := rt.NewObject()
		c := &Client{
			client: http.DefaultClient,
			ctx:    ctx,
			rt:     rt,
		}
		ret.Set("do", c.Do)
		return ret
	}
}

type Client struct {
	client *http.Client
	ctx    context.Context
	rt     *js.Runtime
}

func (c *Client) Do(call js.FunctionCall) js.Value {
	config := RequestConfig{
		Method:     "GET",
		Headers:    make(map[string]string),
		Body:       nil,
		httpClient: c.client,
	}
	if len(call.Arguments) == 0 {
		panic("http.Client.do requires at least 2 arguments")
	}
	if len(call.Arguments) > 0 {
		if err := c.rt.ExportTo(call.Arguments[0], &config.Url); err != nil {
			panic(c.rt.ToValue("http.Client.do invalid url: " + err.Error()))
		}
	}
	if len(call.Arguments) > 1 {
		if err := c.rt.ExportTo(call.Arguments[1], &config); err != nil {
			panic(c.rt.ToValue("http.Client.do invalid config: " + err.Error()))
		}
	}
	if len(call.Arguments) > 2 {
		if err := c.rt.ExportTo(call.Arguments[2], &config.callback); err != nil {
			panic(c.rt.ToValue("http.Client.do invalid callback: " + err.Error()))
		}
	}
	return do(c.ctx, c.rt, config)
}

type RequestConfig struct {
	Url     string            `json:"-"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`

	httpClient *http.Client `json:"-"`
	callback   js.Callable  `json:"-"`
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
		config.httpClient = http.DefaultClient

		ret := rt.NewObject()
		ret.Set("url", config.Url)
		ret.Set("method", config.Method)
		ret.Set("do", requestDo(ctx, rt, config))
		return ret
	}
}

func requestDo(ctx context.Context, rt *js.Runtime, reqConf RequestConfig) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		if len(call.Arguments) == 0 {
			panic("http.request.do requires at least 1 argument")
		}
		if err := rt.ExportTo(call.Arguments[0], &reqConf.callback); err != nil {
			panic(rt.ToValue("http.request.do invalid callback: " + err.Error()))
		}
		return do(ctx, rt, reqConf)
	}
}

func do(_ context.Context, rt *js.Runtime, reqConf RequestConfig) js.Value {
	var reqBody io.Reader
	switch v := reqConf.Body.(type) {
	case string:
		reqBody = strings.NewReader(v)
	default:
		reqBody = nil
	}

	responseObj := rt.NewObject()
	httpRequest, httpErr := http.NewRequest(strings.ToUpper(reqConf.Method), reqConf.Url, reqBody)
	var httpResponse *http.Response
	if httpErr == nil {
		for k, v := range reqConf.Headers {
			httpRequest.Header.Set(k, v)
		}
		if reqConf.Method == "POST" || reqConf.Method == "PUT" {
			httpRequest.Body = io.NopCloser(reqBody)
		}
		if rsp, err := reqConf.httpClient.Do(httpRequest); err != nil {
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
	responseObj.Set("method", reqConf.Method)
	responseObj.Set("url", reqConf.Url)
	responseObj.Set("error", responseError(httpErr, rt))

	if reqConf.callback != nil {
		responseObj.Set("text", responseText(httpResponse, rt))
		responseObj.Set("blob", responseBlob(httpResponse, rt))
		responseObj.Set("json", responseJson(httpResponse, rt))
		responseObj.Set("csv", responseCsv(httpResponse, rt))
		if _, e := reqConf.callback(js.Undefined(), responseObj); e != nil {
			return rt.NewGoError(fmt.Errorf("HTTPError %s", e.Error()))
		}
	}
	return responseObj
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
