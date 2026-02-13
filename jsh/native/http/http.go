package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("NewClient", NewClient)
	m.Set("NewRequest", NewRequest)

	m.Set("NewServer", NewServer)

	m.Set("status", statusCodes)
}

func NewClient() *Client {
	return &Client{
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					if strings.HasPrefix(addr, "unix:") {
						socketPath, err := resolveUnixSocketPath(addr)
						if err != nil {
							return nil, err
						}
						var dialer net.Dialer
						return dialer.DialContext(ctx, "unix", socketPath)
					} else {
						var dialer net.Dialer
						return dialer.DialContext(ctx, network, addr)
					}
				},
			},
		},
	}
}

func resolveUnixSocketPath(addr string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(addr, "unix://../") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(filepath.Dir(pwd), addr[len("unix://../"):]))
	} else if strings.HasPrefix(addr, "../") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(filepath.Dir(pwd), addr[len("../"):]))
	} else if strings.HasPrefix(addr, "unix://./") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(pwd, addr[len("unix://./"):]))
	} else if strings.HasPrefix(addr, "./") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(pwd, addr[len("./"):]))
	} else if strings.HasPrefix(addr, "/") {
		addr = fmt.Sprintf("unix://%s", addr)
	}

	path := strings.TrimPrefix(addr, "unix://")
	if !filepath.IsAbs(path) {
		path = filepath.Join(pwd, path)
	}
	path = filepath.Clean(path)
	return path, nil
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

func (b *Response) Json() (any, error) {
	dec := json.NewDecoder(b.rsp.Body)
	var result any
	if err := dec.Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (b *Response) String() (string, error) {
	data, err := io.ReadAll(b.rsp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

var statusCodes = map[string]int{
	"OK":                            http.StatusOK,                            // 200
	"Created":                       http.StatusCreated,                       // 201
	"Accepted":                      http.StatusAccepted,                      // 202
	"NonAuthoritativeInfo":          http.StatusNonAuthoritativeInfo,          // 203
	"NoContent":                     http.StatusNoContent,                     // 204
	"ResetContent":                  http.StatusResetContent,                  // 205
	"PartialContent":                http.StatusPartialContent,                // 206
	"MultipleChoices":               http.StatusMultipleChoices,               // 300
	"MovedPermanently":              http.StatusMovedPermanently,              // 301
	"Found":                         http.StatusFound,                         // 302
	"SeeOther":                      http.StatusSeeOther,                      // 303
	"NotModified":                   http.StatusNotModified,                   // 304
	"UseProxy":                      http.StatusUseProxy,                      // 305
	"TemporaryRedirect":             http.StatusTemporaryRedirect,             // 307
	"PermanentRedirect":             http.StatusPermanentRedirect,             // 308
	"BadRequest":                    http.StatusBadRequest,                    // 400
	"Unauthorized":                  http.StatusUnauthorized,                  // 401
	"PaymentRequired":               http.StatusPaymentRequired,               // 402
	"Forbidden":                     http.StatusForbidden,                     // 403
	"NotFound":                      http.StatusNotFound,                      // 404
	"MethodNotAllowed":              http.StatusMethodNotAllowed,              // 405
	"NotAcceptable":                 http.StatusNotAcceptable,                 // 406
	"ProxyAuthRequired":             http.StatusProxyAuthRequired,             // 407
	"RequestTimeout":                http.StatusRequestTimeout,                // 408
	"Conflict":                      http.StatusConflict,                      // 409
	"Gone":                          http.StatusGone,                          // 410
	"LengthRequired":                http.StatusLengthRequired,                // 411
	"PreconditionFailed":            http.StatusPreconditionFailed,            // 412
	"RequestEntityTooLarge":         http.StatusRequestEntityTooLarge,         // 413
	"RequestURITooLong":             http.StatusRequestURITooLong,             // 414
	"UnsupportedMediaType":          http.StatusUnsupportedMediaType,          // 415
	"RequestedRangeNotSatisfiable":  http.StatusRequestedRangeNotSatisfiable,  // 416
	"ExpectationFailed":             http.StatusExpectationFailed,             // 417
	"Teapot":                        http.StatusTeapot,                        // 418
	"UnprocessableEntity":           http.StatusUnprocessableEntity,           // 422
	"Locked":                        http.StatusLocked,                        // 423
	"FailedDependency":              http.StatusFailedDependency,              // 424
	"TooEarly":                      http.StatusTooEarly,                      // 425
	"UpgradeRequired":               http.StatusUpgradeRequired,               // 426
	"PreconditionRequired":          http.StatusPreconditionRequired,          // 428
	"TooManyRequests":               http.StatusTooManyRequests,               // 429
	"RequestHeaderFieldsTooLarge":   http.StatusRequestHeaderFieldsTooLarge,   // 431
	"UnavailableForLegalReasons":    http.StatusUnavailableForLegalReasons,    // 451
	"InternalServerError":           http.StatusInternalServerError,           // 500
	"NotImplemented":                http.StatusNotImplemented,                // 501
	"BadGateway":                    http.StatusBadGateway,                    // 502
	"ServiceUnavailable":            http.StatusServiceUnavailable,            // 503
	"GatewayTimeout":                http.StatusGatewayTimeout,                // 504
	"HTTPVersionNotSupported":       http.StatusHTTPVersionNotSupported,       // 505
	"VariantAlsoNegotiates":         http.StatusVariantAlsoNegotiates,         // 506
	"InsufficientStorage":           http.StatusInsufficientStorage,           // 507
	"LoopDetected":                  http.StatusLoopDetected,                  // 508
	"NotExtended":                   http.StatusNotExtended,                   // 510
	"NetworkAuthenticationRequired": http.StatusNetworkAuthenticationRequired, // 511
}
