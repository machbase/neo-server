package http

import (
	"context"
	"net/http"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/gin-gonic/gin"
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/http")
		o := module.Get("exports").(*js.Object)
		// http.request("http://host:port/path", {method: "GET"})
		o.Set("request", request(ctx, rt))
		// cli = new http.Client()
		o.Set("Client", new_client(ctx, rt))
		// lsnr = new http.Listener({network:'tcp', address:'127.0.0.1:9001'})
		// lsnr = new http.Listener() // share the system default http server
		o.Set("Server", new_server(ctx, rt))
		// status.OK
		o.Set("status", statusCodes)
	}
}

var defaultRouter *gin.Engine

func SetDefaultRouter(router *gin.Engine) {
	defaultRouter = router
}

func DefaultRouter() *gin.Engine {
	return defaultRouter
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
