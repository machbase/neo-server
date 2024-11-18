package httpd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/service/msg"
	"github.com/machbase/neo-server/v8/mods/util"
)

func (svr *httpd) handleWatchQuery(ctx *gin.Context) {
	tick := time.Now()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Watcher panic", r)
		}
	}()

	var period time.Duration
	if p, err := time.ParseDuration(ctx.Query("period")); err == nil {
		period = p
	}
	if period < 1*time.Second {
		period = 1 * time.Second
	}
	var keepAlive time.Duration
	if p, err := time.ParseDuration(ctx.Query("keep-alive")); err == nil {
		keepAlive = p
	}
	if keepAlive == 0 {
		keepAlive = 30 * time.Second
	}

	var maxRowNum = strInt(ctx.Query("max-rows"), 100)
	var parallelism = strInt(ctx.Query("parallelism"), 3)

	timeformat := strString(ctx.Query("timeformat"), "ns")
	tz := time.UTC
	if timezone := ctx.Query("tz"); timezone != "" {
		tz, _ = util.ParseTimeLocation(timezone, time.UTC)
	}

	watch, err := api.NewWatcher(ctx,
		api.WatcherConfig{
			ConnProvider: func() (api.Conn, error) { return svr.getTrustConnection(ctx) },
			TableName:    ctx.Param("table"),
			TagNames:     ctx.QueryArray("tag"),
			Timeformat:   timeformat,
			Timezone:     tz,
			Parallelism:  parallelism,
			ChanSize:     100,
			MaxRowNum:    maxRowNum,
		})
	if err != nil {
		svr.log.Debug("Watcher error", err.Error())
		rsp := msg.QueryResponse{Reason: err.Error(), Elapse: time.Since(tick).String()}
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	defer watch.Close()

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	periodTick := time.NewTicker(period)
	defer periodTick.Stop()
	keepAliveTick := time.NewTicker(keepAlive)
	defer keepAliveTick.Stop()

	lastWriteTime := time.Now()
	svr.log.Infof("%s start period %v, keep-alive %v", watch.String(), period, keepAlive)
	watch.Execute()
	for {
		select {
		case <-keepAliveTick.C:
			if time.Since(lastWriteTime) < keepAlive {
				continue
			}
			ctx.Writer.Write([]byte(": keep-alive\n\n"))
			ctx.Writer.Flush()
			lastWriteTime = time.Now()
		case <-periodTick.C:
			watch.Execute()
		case data := <-watch.C:
			switch v := data.(type) {
			case api.WatchData:
				b, _ := json.Marshal(v)
				ctx.Writer.Write([]byte("data: "))
				ctx.Writer.Write(b)
				ctx.Writer.Write([]byte("\n\n"))
				ctx.Writer.Flush()
				lastWriteTime = time.Now()
			case error:
				ctx.Writer.Write([]byte(fmt.Sprintf("error: %s\n\n", v.Error())))
				ctx.Writer.Flush()
				lastWriteTime = time.Now()
			}
		case <-ctx.Writer.CloseNotify():
			svr.log.Infof("%s end", watch.String())
			return
		}
	}
}
