package server

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OutOfBedlam/metric"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/api"
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

func MetricsInterceptor() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		latency := time.Since(start)
		m := metric.Measurement{Name: "http"}
		m.AddField(metric.Field{Name: "count", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
		m.AddField(metric.Field{Name: "latency", Value: float64(latency.Nanoseconds()), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)})
		if strings.HasPrefix(c.Request.URL.Path, "/db/write") {
			m.AddField(metric.Field{Name: "write:count", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
			m.AddField(metric.Field{Name: "write:latency", Value: float64(latency.Nanoseconds()), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)})
		} else if strings.HasPrefix(c.Request.URL.Path, "/db/query") {
			m.AddField(metric.Field{Name: "query:count", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
			m.AddField(metric.Field{Name: "query:latency", Value: float64(latency.Nanoseconds()), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)})
		} else if strings.HasPrefix(c.Request.URL.Path, "/db/tql") {
			m.AddField(metric.Field{Name: "tql:count", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
			m.AddField(metric.Field{Name: "tql:latency", Value: float64(latency.Nanoseconds()), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)})
		}
		if s := c.Request.ContentLength; s > 0 {
			m.AddField(metric.Field{Name: "recv_bytes", Value: float64(s), Unit: metric.UnitBytes, Type: metric.FieldTypeCounter})
		}
		if s := c.Writer.Size(); s > 0 {
			m.AddField(metric.Field{Name: "send_bytes", Value: float64(s), Unit: metric.UnitBytes, Type: metric.FieldTypeCounter})
		}

		status := c.Writer.Status()
		if status < 200 {
			m.AddField(metric.Field{Name: "status_1xx", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
		} else if status < 300 {
			m.AddField(metric.Field{Name: "status_2xx", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
		} else if status < 400 {
			m.AddField(metric.Field{Name: "status_3xx", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
		} else if status < 500 {
			m.AddField(metric.Field{Name: "status_4xx", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
		} else {
			m.AddField(metric.Field{Name: "status_5xx", Value: 1, Unit: metric.UnitShort, Type: metric.FieldTypeCounter})
		}
		api.AddMetrics(m)
	}
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

func HttpLogger(loggingName string, logEnabled *bool, logLatencyThreshold *time.Duration) gin.HandlerFunc {
	return HttpLoggerWithFilter(loggingName, func(req *http.Request, statusCode int, latency time.Duration) bool {
		// when log is disabled
		if logEnabled == nil || !*logEnabled {
			return false
		}
		// when status code is error
		if statusCode >= 400 {
			return true
		}
		// when logLatencyThreshold is not set
		if logLatencyThreshold == nil || *logLatencyThreshold < 0 {
			return false
		}

		// when logLatencyThreshold is set
		return latency >= *logLatencyThreshold
	})
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

var ignoreAccessLog = []struct {
	pathSuffix string
	method     string
}{
	{pathSuffix: "/healthz", method: http.MethodGet},
	{pathSuffix: "/statz", method: http.MethodGet},
	{pathSuffix: "/web/api/check", method: http.MethodGet},
}

func logger(log logging.Log, filter HttpLoggerFilter) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		for _, ignore := range ignoreAccessLog {
			if c.Request.Method == ignore.method && strings.HasSuffix(c.Request.URL.Path, ignore.pathSuffix) {
				return
			}
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

		wSize := c.Writer.Size()
		if wSize == -1 {
			wSize = 0
		}
		WriteSize := util.HumanizeByteCount(int64(wSize))
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

type WsReadWriter struct {
	*websocket.Conn
	r  io.Reader
	mu sync.Mutex
}

var _ io.ReadWriter = (*WsReadWriter)(nil)

func (ws *WsReadWriter) Read(p []byte) (int, error) {
	if ws.r == nil {
		if _, r, err := ws.NextReader(); err != nil {
			return 0, err
		} else {
			ws.r = r
		}
	}
	n, err := ws.r.Read(p)
	if err == io.EOF {
		if _, r, err := ws.NextReader(); err != nil {
			return 0, err
		} else {
			ws.r = r
		}
		m, e := ws.r.Read(p[n:])
		n += m
		err = e
	}
	return n, err
}

func (ws *WsReadWriter) Write(data []byte) (int, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	err := (*ws).WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}
