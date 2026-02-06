package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/dop251/goja"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("NewClient", NewClient)
	m.Set("NewRequest", NewRequest)
}

func NewClient() *Client {
	return &Client{
		client: &http.Client{},
	}
}

type Client struct {
	client *http.Client
}

func (agent *Client) Do(req *Request) (*Response, error) {
	rsp, err := agent.client.Do(req.Request)
	if err != nil {
		return nil, err
	}
	ret := NewResponse(rsp)
	// IMPORTANT: Caller is responsible for closing the response body
	// see Response.Close()
	return ret, nil
}

type Request struct {
	*http.Request
}

func NewRequest(method string, url string) (*Request, error) {
	ret := &Request{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	ret.Request = req
	return ret, nil
}

func (r *Request) WriteString(s string, encoding string) (n int, err error) {
	return r.Write([]byte(s))
}

func (r *Request) Write(p []byte) (n int, err error) {
	if r.Body == nil {
		r.Body = io.NopCloser(bytes.NewReader(p))
	} else {
		// Append to existing body
		buf := new(bytes.Buffer)
		_, err := io.Copy(buf, r.Body)
		if err != nil {
			return 0, err
		}
		buf.Write(p)
		r.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
	}
	return len(p), nil
}

type Response struct {
	rsp           *http.Response
	Ok            bool
	Proto         string
	ProtoMajor    int
	ProtoMinor    int
	StatusCode    int
	StatusMessage string
	Headers       map[string]any
}

func NewResponse(rsp *http.Response) *Response {
	headers := map[string]any{}
	for k, v := range rsp.Header {
		if len(v) == 1 {
			headers[k] = v[0]
		} else {
			headers[k] = v
		}
	}
	return &Response{
		rsp:           rsp,
		Ok:            rsp.StatusCode >= 200 && rsp.StatusCode < 300,
		Proto:         rsp.Proto,
		ProtoMajor:    rsp.ProtoMajor,
		ProtoMinor:    rsp.ProtoMinor,
		StatusCode:    rsp.StatusCode,
		StatusMessage: rsp.Status,
		Headers:       headers,
	}
}

func (b *Response) Close() error {
	return b.rsp.Body.Close()
}

func (b *Response) Json() map[string]any {
	dec := json.NewDecoder(b.rsp.Body)
	var result map[string]any
	if err := dec.Decode(&result); err != nil {
		return nil
	}
	return result
}

func (b *Response) String() string {
	data, err := io.ReadAll(b.rsp.Body)
	if err != nil {
		return ""
	}
	return string(data)
}
