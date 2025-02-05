package server

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
)

func strBool(str string, def bool) bool {
	if str == "" {
		return def
	}
	return strings.ToLower(str) == "true" || str == "1"
}

func strInt(str string, def int) int {
	if str == "" {
		return def
	}
	v, err := strconv.Atoi(str)
	if err != nil {
		return def
	}
	return v
}

func strString(str string, def string) string {
	if str == "" {
		return def
	}
	return str
}

var metricRequestTotal = uint64(0)
var metricLatencyUnder1ms = uint64(0)
var metricLatencyUnder100ms = uint64(0)
var metricLatencyUnder1s = uint64(0)
var metricLatencyUnder5s = uint64(0)
var metricLatencyUnder3s = uint64(0)
var metricLatencyOver5s = uint64(0)
var metricRecvContentBytes = uint64(0)
var metricSendContentBytes = uint64(0)
var metricStatus1xx = uint64(0)
var metricStatus2xx = uint64(0)
var metricStatus3xx = uint64(0)
var metricStatus4xx = uint64(0)
var metricStatus5xx = uint64(0)

func MetricsInterceptor() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latencyMillis := time.Since(start).Milliseconds()

		atomic.AddUint64(&metricRequestTotal, 1)
		if latencyMillis <= 1 { // under 1ms
			atomic.AddUint64(&metricLatencyUnder100ms, 1)
		} else if latencyMillis <= 100 { // under 100ms
			atomic.AddUint64(&metricLatencyUnder100ms, 1)
		} else if latencyMillis <= 1000 { // under 1s
			atomic.AddUint64(&metricLatencyUnder1s, 1)
		} else if latencyMillis <= 3000 { // under 3s
			atomic.AddUint64(&metricLatencyUnder3s, 1)
		} else if latencyMillis <= 5000 { // under 5s
			atomic.AddUint64(&metricLatencyUnder5s, 1)
		} else { // over 1s
			atomic.AddUint64(&metricLatencyOver5s, 1)
		}
		if s := c.Request.ContentLength; s > 0 {
			atomic.AddUint64(&metricRecvContentBytes, uint64(s))
		}
		if s := c.Writer.Size(); s > 0 {
			atomic.AddUint64(&metricSendContentBytes, uint64(s))
		}

		status := c.Writer.Status()
		if status < 200 {
			atomic.AddUint64(&metricStatus1xx, 1)
		} else if status < 300 {
			atomic.AddUint64(&metricStatus2xx, 1)
		} else if status < 400 {
			atomic.AddUint64(&metricStatus3xx, 1)
		} else if status < 500 {
			atomic.AddUint64(&metricStatus4xx, 1)
		} else {
			atomic.AddUint64(&metricStatus5xx, 1)
		}
	}
}

func Metrics() map[string]any {
	ret := make(map[string]any)
	ret["request_total"] = atomic.LoadUint64(&metricRequestTotal)
	ret["latency_1ms"] = atomic.LoadUint64(&metricLatencyUnder1ms)
	ret["latency_100ms"] = atomic.LoadUint64(&metricLatencyUnder100ms)
	ret["latency_1s"] = atomic.LoadUint64(&metricLatencyUnder1s)
	ret["latency_3s"] = atomic.LoadUint64(&metricLatencyUnder3s)
	ret["latency_5s"] = atomic.LoadUint64(&metricLatencyUnder5s)
	ret["latency_over_5s"] = atomic.LoadUint64(&metricLatencyOver5s)
	ret["bytes_recv"] = atomic.LoadUint64(&metricRecvContentBytes)
	ret["bytes_send"] = atomic.LoadUint64(&metricSendContentBytes)
	ret["status_1xx"] = atomic.LoadUint64(&metricStatus1xx)
	ret["status_2xx"] = atomic.LoadUint64(&metricStatus2xx)
	ret["status_3xx"] = atomic.LoadUint64(&metricStatus3xx)
	ret["status_4xx"] = atomic.LoadUint64(&metricStatus4xx)
	ret["status_5xx"] = atomic.LoadUint64(&metricStatus5xx)
	return ret
}

func RecoveryWithLogging(log logging.Log, recovery ...gin.RecoveryFunc) gin.HandlerFunc {
	gin.DefaultWriter = log
	gin.DefaultErrorWriter = log

	if len(recovery) > 0 {
		return gin.CustomRecoveryWithWriter(log, recovery[0])
	}
	return gin.CustomRecoveryWithWriter(log, func(c *gin.Context, err any) {
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

type HttpLoggerFilter func(req *http.Request, statusCode int, latency time.Duration) bool

func HttpLogger(loggingName string) gin.HandlerFunc {
	return HttpLoggerWithFilter(loggingName, nil)
}

func HttpLoggerWithFilter(loggingName string, filter HttpLoggerFilter) gin.HandlerFunc {
	log := logging.GetLog(loggingName)
	return logger(log, filter)
}

func HttpLoggerWithFile(loggingName string, filename string) gin.HandlerFunc {
	return HttpLoggerWithFileConf(loggingName,
		logging.LogFileConf{
			Filename:             filename,
			Level:                "DEBUG",
			MaxSize:              10,
			MaxBackups:           2,
			MaxAge:               7,
			Compress:             false,
			Append:               true,
			RotateSchedule:       "@midnight",
			Console:              false,
			PrefixWidth:          20,
			EnableSourceLocation: false,
		})
}

func HttpLoggerWithFileConf(loggingName string, fileConf logging.LogFileConf) gin.HandlerFunc {
	return HttpLoggerWithFilterAndFileConf(loggingName, nil, fileConf)
}

func HttpLoggerWithFilterAndFileConf(loggingName string, filter HttpLoggerFilter, fileConf logging.LogFileConf) gin.HandlerFunc {
	if len(fileConf.Filename) > 0 {
		return logger(logging.NewLogFile(loggingName, fileConf), filter)
	} else {
		return HttpLoggerWithFilter(loggingName, filter)
	}
}

func logger(log logging.Log, filter HttpLoggerFilter) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// ignore health checker
		if strings.HasSuffix(c.Request.URL.Path, "/healthz") && c.Request.Method == http.MethodGet {
			return
		}

		// Stop timer
		TimeStamp := time.Now()
		Latency := TimeStamp.Sub(start)

		StatusCode := c.Writer.Status()

		// filter exists, and it returns false not to leave log
		if filter != nil && !filter(c.Request, StatusCode, Latency) {
			return
		}

		url := c.Request.Host + c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if len(raw) > 0 {
			url = url + "?" + raw
		}

		ClientIP := c.ClientIP()
		Proto := c.Request.Proto
		Method := c.Request.Method
		ErrorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()
		if len(ErrorMessage) > 0 {
			ErrorMessage = "\n" + ErrorMessage
		}

		wsize := c.Writer.Size()
		if wsize == -1 {
			wsize = 0
		}
		WriteSize := util.HumanizeByteCount(int64(wsize))
		ReadSize := util.HumanizeByteCount(c.Request.ContentLength)

		color := ""
		reset := "\033[0m"
		level := logging.LevelDebug

		switch {
		case StatusCode >= http.StatusContinue && StatusCode < http.StatusOK:
			color, reset = "", "" // 1xx
		case StatusCode >= http.StatusOK && StatusCode < http.StatusMultipleChoices:
			color = "\033[97;42m" // 2xx green
		case StatusCode >= http.StatusMultipleChoices && StatusCode < http.StatusBadRequest:
			color = "\033[90;47m" // 3xx white
		case StatusCode >= http.StatusBadRequest && StatusCode < http.StatusInternalServerError:
			color = "\033[90;43m" // 4xx yellow
		default:
			color = "\033[97;41m" // 5xx red
			level = logging.LevelError
		}

		log.Logf(level, "%s %3d %s| %13v | %15s | %8s | %8s | %s %-7s %s%s",
			color, StatusCode, reset,
			Latency,
			ClientIP,
			ReadSize,
			WriteSize,
			Proto,
			Method,
			url,
			ErrorMessage,
		)
	}
}
