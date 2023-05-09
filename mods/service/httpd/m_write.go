package httpd

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/do"
	spi "github.com/machbase/neo-spi"
)

type lakeReq struct {
	TagName string          `json:"tagName"`
	Values  [][]interface{} `json:"values"`
}

type lakeRsp struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason,omitempty"`
	Data    string `json:"data,omitempty"`
}

var once sync.Once
var appender spi.Appender

const tableName = "TAG"

func (svr *httpd) handleAppender(ctx *gin.Context) {
	rsp := lakeRsp{Success: false}

	req := lakeReq{}
	err := ctx.Bind(&req)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.TagName == "" {
		rsp.Reason = "tag name is empty"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.Values == nil || len(req.Values) == 0 {
		rsp.Reason = "values is nil"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	// log.Printf("req : %+v\n", req)

	once.Do(func() {
		exists, err := do.ExistsTable(svr.db, tableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusPreconditionFailed, rsp)
			return
		}

		if !exists {
			rsp.Reason = fmt.Sprintf("%s table is not exist", tableName)
			ctx.JSON(http.StatusPreconditionFailed, rsp)
			return
		}

		appender, err = svr.db.Appender(tableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}

		// close 시점 언제?
		defer appender.Close()
	})

	if appender == nil {
		log.Println("appender is nil")
		appender, err = svr.db.Appender(tableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	dataSet := make([][]interface{}, len(req.Values))
	for idx, value := range req.Values {
		temp := []interface{}{req.TagName}
		// 임시
		t, _ := time.Parse("2006-01-02 15:04:05", value[0].(string))
		value[0] = t
		dataSet[idx] = append(temp, value...)
	}

	//  req.values, data set ([[time, value, ext_value, ...], [time, value, ext_value, ...], ...])
	for _, data := range dataSet {
		err = appender.Append(data...)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	rsp.Success = true
	ctx.JSON(http.StatusOK, rsp)
}

//=================================

type queryRequest struct {
	EdgeId    string
	StartTime string
	EndTime   string
	Offset    string
	Limit     string
	Level     string
	Job       string
	Keyword   string
	//filename string
}

type queryResponse struct {
	Success bool     `json:"success"`
	Reason  string   `json:"reason,omitempty"`
	Lines   []string `json:"lines"`
}

func (svr *httpd) handleLogs(ctx *gin.Context) {
	rsp := queryResponse{Success: false}

	req := queryRequest{}
	if ctx.Request.Method == http.MethodGet {
		req.EdgeId = ctx.Query("edgeId")
		req.StartTime = ctx.Query("startTime") // strString() -> default?
		req.EndTime = ctx.Query("endTime")
		req.Level = ctx.Query("level")
		req.Limit = ctx.Query("limit")
		req.Offset = ctx.Query("offset")
		req.Job = ctx.Query("job")
		req.Keyword = ctx.Query("keyword")

		// if req.Keyword == "" || req.StartTime == "" || req.EndTime == "" { // 일단 이 3개만
		// 	rsp.Reason = "request empty"
		// 	ctx.JSON(http.StatusOK, rsp)
		// 	return
		// }
	} else {
		rsp.Reason = fmt.Sprintf("unsupported method %s", ctx.Request.Method)
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	params := []string{}

	// table check? 테이블 이름은 고정되는가?

	sqlText := "SELECT line FROM logdata WHERE"

	andFlag := false
	if req.EdgeId != "" {
		sqlText += fmt.Sprintf("edgeid = ?", req.EdgeId) // 뒤에 if 문들을 실행하게 되면 뒤에 AND 필요
		params = append(params, req.EdgeId)
		andFlag = true
	}
	if req.StartTime != "" {
		if andFlag {
			sqlText += "AND "
			andFlag = false
		}
		sqlText += fmt.Sprintf("edgeid = ?", req.StartTime)
		params = append(params, req.EdgeId)
		andFlag = true
	}
	if req.EndTime != "" {
		sqlText += "AND "
		sqlText += fmt.Sprintf("edgeid = ?", req.StartTime)
		params = append(params, req.EdgeId)
	}
	if req.Offset != "" {
		sqlText += "AND "
		sqlText += fmt.Sprintf("offset = ?", req.StartTime)
		params = append(params, req.Offset)
	}
	if req.Limit != "" {
		sqlText += "AND "
		sqlText += fmt.Sprintf("limit = ?", req.StartTime)
		params = append(params, req.Limit)
	}
	if req.Level != "" {
		sqlText += "AND "
		sqlText += fmt.Sprintf("level = ?", req.StartTime)
		params = append(params, req.EdgeId)
	}

	if req.Job != "" {
		sqlText += "AND "
		sqlText += fmt.Sprintf("job = ?", req.StartTime)
		params = append(params, req.Job)
	} else {

	}

	if req.Keyword != "" {
		sqlText += "line search ?"
		params = append(params, req.Keyword)
	}

	rows, err := svr.db.Query(sqlText, params)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	for rows.Next() {
		line := ""
		err = rows.Scan(&line)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		rsp.Lines = append(rsp.Lines, line)
	}

	rsp.Success = true
	ctx.JSON(http.StatusOK, rsp)
}



'TST_MCHDV_111111111'  , '2023-05-03 17:48:03.006' , syslogs  , authlogs    , 1     , machbase is so good!                                                                                                                                                             |
'TST_MCHDV_111111111'  , '2023-05-03 17:48:03.006' , syslogs  , historylogs , 1     , machbase is so good!                                                                                                                                                             |
'TST_MCHDV_111111111'  , '2023-05-03 17:48:03.006' , syslogs  , systemlogs  , 1     , machbase is so good!                                                                                                                                                             |
'TST_MCHDV_111111111'  , '2023-05-03 17:48:03.006' , syslogs  , systemlogs  , 2     , machbase is good!                                                                                                                                                                |
'TST_MCHDV_1234567890' , '2023-05-03 17:58:16.124' , messages , varlogs     , 1     , May  3 17:58:15 DCU-KEPCO user.info kernel: [1147757.004577] dm9620 2-1:1.0: eth1: link up, 100Mbps, full-duplex, lpa 0x0561                                                     |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:06.424' , messages , varlogs     , 1     , May  3 17:57:06 DCU-KEPCO user.info kernel: [1147687.283996] dm9620 2-1:1.0: eth1: link down                                                                                     |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:05.421' , messages , varlogs     , 1     , May  3 17:57:05 DCU-KEPCO user.info kernel: [1147686.192413] dm9620 2-1:1.0: eth1: link down                                                                                     |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:04.915' , messages , varlogs     , 1     , May  3 17:57:04 DCU-KEPCO user.info kernel: [1147685.700347] dm9620 2-1:1.0: eth1: register 'dm9620' at usb-musb-hdrc.1-1, Davicom DM96xx USB 10/100 Ethernet, 00:00:ff:ff:00:00 |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:04.915' , messages , varlogs     , 1     , May  3 17:57:04 DCU-KEPCO user.info kernel: [1147685.685394] usb 2-1: SerialNumber: 1                                                                                            |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:04.915' , messages , varlogs     , 1     , May  3 17:57:04 DCU-KEPCO user.info kernel: [1147685.681457] usb 2-1: Manufacturer:                                                                                              |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:04.915' , messages , varlogs     , 1     , May  3 17:57:04 DCU-KEPCO user.info kernel: [1147685.677398] usb 2-1: Product: USB Eth                                                                                           |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:04.915' , messages , varlogs     , 1     , May  3 17:57:04 DCU-KEPCO user.info kernel: [1147685.669647] usb 2-1: New USB device strings: Mfr=1, Product=2, SerialNumber=3                                                   |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:04.915' , messages , varlogs     , 1     , May  3 17:57:04 DCU-KEPCO user.info kernel: [1147685.662384] usb 2-1: New USB device found, idVendor=0a46, idProduct=9620                                                        |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:04.664' , messages , varlogs     , 1     , May  3 17:57:04 DCU-KEPCO user.warn kernel: [1147685.650268] usb 2-1: config 1 interface 0 altsetting 0 endpoint 0x83 has an invalid bInterval 0, changing to 9                  |
'TST_MCHDV_1234567890' , '2023-05-03 17:57:04.664' , messages , varlogs     , 1     , May  3 17:57:04 DCU-KEPCO user.info kernel: [1147685.510040] usb 2-1: new high-speed USB device number 69 using musb-hdrc                                                        |
'TST_MCHDV_1234567890' , 2023-05-03 17:57:03.142 , messages , varlogs     , 1     , May  3 17:57:03 DCU-KEPCO user.info kernel: [1147684.124755] dm9620 2-1:1.0: eth1: unregister 'dm9620' usb-musb-hdrc.1-1, Davicom DM96xx USB 10/100 Ethernet                     |
'TST_MCHDV_1234567890' , 2023-05-03 17:57:03.142 , messages , varlogs     , 1     , May  3 17:57:03 DCU-KEPCO user.info kernel: [1147684.118225] usb 2-1: USB disconnect, device number 68                                                                           |
'TST_MCHDV_1234567890' , 2023-05-03 17:57:03.142 , messages , varlogs     , 1     , May  3 17:57:02 DCU-KEPCO user.info kernel: [1147683.949645] [cpsw_private_ioctl] Write result mismatch (0x3300 ==> 0x3100)                                                      |
'TST_MCHDV_1234567890' , 2023-05-03 17:57:01.388 , messages , varlogs     , 1     , May  3 17:57:01 DCU-KEPCO user.info kernel: [1147682.196685] [cpsw_private_ioctl] Write result mismatch (0x175D ==> 0x0000)                                                      |
'TST_MCHDV_1234567890' , 2023-05-03 17:48:03.06  , messages , varlogs     , 1     , May  3 17:48:02 DCU-KEPCO authpriv.info dropbear[16463]: Exit (root): Error reading: Connection timed out                                                                        |
'TST_MCHDV_1234567890' , 2023-05-03 16:37:28.718 , messages , varlogs     , 1     , May  3 16:37:28 DCU-KEPCO authpriv.warn dropbear[9130]: lastlog_openseek: /var/log/lastlog is not a file or directory!                                                           |
'TST_MCHDV_1234567890' , 2023-05-03 16:37:28.718 , messages , varlogs     , 1     , May  3 16:37:28 DCU-KEPCO authpriv.warn dropbear[9130]: lastlog_perform_login: Couldn't stat /var/log/lastlog: No such file or directory                                         |
'TST_MCHDV_1234567890' , 2023-05-03 16:37:28.217 , messages , varlogs     , 1     , May  3 16:37:28 DCU-KEPCO authpriv.notice dropbear[9126]: Password auth succeeded for 'root' from 192.168.20.100:1337                                                            |
'TST_MCHDV_1234567890' , 2023-05-03 16:37:27.465 , messages , varlogs     , 1     , May  3 16:37:27 DCU-KEPCO authpriv.info dropbear[9126]: Child connection from 192.168.20.100:1337                                                                                |
'TST_MCHDV_1234567890' , 2023-05-03 16:37:27.464 , messages , varlogs     , 1     , May  3 16:37:27 DCU-KEPCO authpriv.notice dropbear[9103]: Password auth succeeded for 'root' from 192.168.20.100:1334                                                            |
'TST_MCHDV_1234567890' , 2023-05-03 16:37:09.902 , messages , varlogs     , 1     , May  3 16:37:09 DCU-KEPCO authpriv.info dropbear[9103]: Child connection from 192.168.20.100:1334                                                                                |
'TST_MCHDV_1234567890' , 2023-05-03 16:35:12.039 , messages , varlogs     , 1     , May  3 16:35:11 DCU-KEPCO authpriv.info dropbear[8874]: Exit before auth: Exited normally               